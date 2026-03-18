package llmctx

import (
	"context"
	"fmt"
	"sync"

	msg "github.com/quarkloop/agent/pkg/context/message"
)

// =============================================================================
// agentctx.go
//
// AgentContext is the central domain object: an ordered, concurrent-safe
// message window with O(1) token accounting, type-aware visibility filtering,
// pluggable compaction, and throughput tracking.
//
// Construct via AgentContextBuilder (builder.go) — never instantiate directly.
// =============================================================================

// AgentContext manages the ordered message list fed to an LLM agent.
// It is safe for concurrent use from multiple goroutines.
type AgentContext struct {
	mu            sync.RWMutex
	systemPrompt  *Message
	messages      []*Message
	index         map[string]int // MessageID → slice position (O(1) lookup)
	contextWindow ContextWindow
	compactor     Compactor
	tc            TokenComputer
	idGen         IDGenerator      // pluggable ID generation strategy
	cachedTokens  TokenCount       // running total maintained on every mutation
	compact       compactionTracker
	tput          throughputTracker

}

// =============================================================================
// Mutation methods
// =============================================================================

// AppendMessage adds message to the end of the context and updates the index,
// the running token total, and the throughput tracker.
//
// ctx is used for cancellation during write-time middleware execution (semantic
// compression, contradiction detection). Pass context.Background() when no
// deadline is needed.
//
// When write-time middleware is configured:
//   - SemanticCompressor runs first; if an auto-merge fires the message is NOT
//     appended (the existing message is updated instead).
//   - ContradictionDetector runs next; warnings are accumulated in
//     PendingContradictions but the message is always appended.
//
// Returns ErrCodeInvalidMessage for nil input.
// Returns context.Err() when ctx is cancelled before the message is appended.
func (ac *AgentContext) AppendMessage(ctx context.Context, message *Message) error {
	if err := ctx.Err(); err != nil {
		return newErr(ErrCodeInvalidMessage, "AppendMessage: context cancelled", err)
	}
	if message == nil {
		return newErr(ErrCodeInvalidMessage, "cannot append nil message", nil)
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.index[message.id.value] = len(ac.messages)
	ac.messages = append(ac.messages, message)
	ac.cachedTokens = ac.cachedTokens.Add(message.tokenCount)
	ac.tput.recordAppend(message.tokenCount)
	return nil
}

// RemoveMessageByID removes the message with the given ID.
//
// ctx is reserved for future use (e.g. pre-removal hooks) and for consistency
// with the rest of the mutation API. Pass context.Background() when no deadline
// is needed.
//
// Returns ErrCodeSystemPromptLocked when the system prompt is targeted.
// Returns ErrCodeMessageNotFound when the ID is absent.
func (ac *AgentContext) RemoveMessageByID(ctx context.Context, id MessageID) error {
	if err := ctx.Err(); err != nil {
		return newErr(ErrCodeMessageNotFound, "RemoveMessageByID: context cancelled", err)
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()

	pos, ok := ac.index[id.value]
	if !ok {
		return newErr(ErrCodeMessageNotFound, fmt.Sprintf("message %q not found", id), nil)
	}
	if ac.systemPrompt != nil && id.value == ac.systemPrompt.id.value {
		return newErr(ErrCodeSystemPromptLocked, "system prompt cannot be removed", nil)
	}

	removed := ac.messages[pos]
	ac.cachedTokens = ac.cachedTokens.Sub(removed.tokenCount)
	ac.messages = append(ac.messages[:pos], ac.messages[pos+1:]...)
	delete(ac.index, id.value)
	for i := pos; i < len(ac.messages); i++ {
		ac.index[ac.messages[i].id.value] = i
	}
	ac.tput.recordRemove(1)
	return nil
}

// RemoveMessagesByWeight removes all non-system messages with the given weight.
// Returns the number of removed messages.
//
// ctx is checked for cancellation before the operation begins.
func (ac *AgentContext) RemoveMessagesByWeight(ctx context.Context, weight MessageWeight) int {
	if ctx.Err() != nil {
		return 0
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()

	kept := ac.messages[:0]
	n := 0
	for _, m := range ac.messages {
		isSystem := ac.systemPrompt != nil && m.id.value == ac.systemPrompt.id.value
		if m.weight.Equal(weight) && !isSystem {
			ac.cachedTokens = ac.cachedTokens.Sub(m.tokenCount)
			delete(ac.index, m.id.value)
			n++
		} else {
			ac.index[m.id.value] = len(kept)
			kept = append(kept, m)
		}
	}
	ac.messages = kept
	ac.tput.recordRemove(n)
	return n
}

// UpdateMessagePayload replaces the payload on an existing message and keeps
// the running token total consistent. Returns ErrCodeMessageNotFound if absent.
//
// ctx is checked for cancellation before the operation begins.
func (ac *AgentContext) UpdateMessagePayload(ctx context.Context, id MessageID, payload msg.Payload) error {
	if err := ctx.Err(); err != nil {
		return newErr(ErrCodeMessageNotFound, "UpdateMessagePayload: context cancelled", err)
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()

	pos, ok := ac.index[id.value]
	if !ok {
		return newErr(ErrCodeMessageNotFound, fmt.Sprintf("message %q not found", id), nil)
	}
	m := ac.messages[pos]
	oldTokens, err := m.SetPayload(payload, ac.tc)
	if err != nil {
		return err
	}
	ac.cachedTokens = ac.cachedTokens.Sub(oldTokens).Add(m.tokenCount)
	return nil
}

// ApplyPolicy re-applies a VisibilityPolicy to every message currently in the
// context by replacing each message with a visibility-overridden copy.
//
// Use this when the policy changes mid-session (e.g. the user enables a
// "show tool activity" toggle) so existing messages immediately reflect the
// new settings.
//
// Note: messages whose type is not in the policy retain their current visibility.
func (ac *AgentContext) ApplyPolicy(policy *VisibilityPolicy) {
	if policy == nil {
		return
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()
	for i, m := range ac.messages {
		newVis := policy.For(m.Type())
		ac.messages[i] = m.WithVisibility(newVis)
	}
}

// Compact runs the configured compaction strategy and records the event.
//
// ctx is checked for cancellation before the operation begins. For compactors
// that make external calls (future), ctx will be threaded through.
//
// Returns ErrCodeNoCompactor when no compactor has been set.
func (ac *AgentContext) Compact(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return newErr(ErrCodeCompactionFailed, "Compact: context cancelled", err)
	}
	if ac.compactor == nil {
		return newErr(ErrCodeNoCompactor,
			"no compactor configured; set one via AgentContextBuilder.WithCompactor", nil)
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()

	before := ac.cachedTokens
	beforeCount := int32(len(ac.messages))

	compacted, err := ac.compactor.Compact(ac.messages, ac.contextWindow)
	if err != nil {
		return newErr(ErrCodeCompactionFailed, "compaction failed", err)
	}

	// Rebuild index and running total in one pass.
	ac.messages = compacted
	ac.index = make(map[string]int, len(compacted))
	ac.cachedTokens = TokenCount{}
	for i, m := range compacted {
		ac.index[m.id.value] = i
		ac.cachedTokens = ac.cachedTokens.Add(m.tokenCount)
	}
	removed := beforeCount - int32(len(compacted))
	ac.compact.record(before, ac.cachedTokens, removed)
	ac.tput.recordRemove(int(removed))
	return nil
}

// =============================================================================
// Query methods
// =============================================================================

// FindMessage returns the message with the given ID.
// Returns ErrCodeMessageNotFound when absent.
func (ac *AgentContext) FindMessage(id MessageID) (*Message, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	if pos, ok := ac.index[id.value]; ok {
		return ac.messages[pos], nil
	}
	return nil, newErr(ErrCodeMessageNotFound, fmt.Sprintf("message %q not found", id), nil)
}

// Messages returns a shallow copy of every message, preserving insertion order.
func (ac *AgentContext) Messages() []*Message {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	out := make([]*Message, len(ac.messages))
	copy(out, ac.messages)
	return out
}

// VisibleMessages returns only messages whose visibility includes target.
//
//	ac.VisibleMessages(llmctx.VisibleToUser)      → user-facing messages
//	ac.VisibleMessages(llmctx.VisibleToLLM)       → LLM payload
//	ac.VisibleMessages(llmctx.VisibleToDeveloper) → debug view
func (ac *AgentContext) VisibleMessages(target Visibility) []*Message {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	out := make([]*Message, 0, len(ac.messages))
	for _, m := range ac.messages {
		if m.IsVisibleTo(target) {
			out = append(out, m)
		}
	}
	return out
}

// LLMMessages is a convenience wrapper for VisibleMessages(VisibleToLLM).
// These are the messages that adapters should serialise and send to the LLM.
func (ac *AgentContext) LLMMessages() []*Message {
	return ac.VisibleMessages(VisibleToLLM)
}

// FilterMessages returns messages matching all provided predicates.
// Predicates are ANDed: a message is included only when every predicate returns true.
func (ac *AgentContext) FilterMessages(predicates ...func(*Message) bool) []*Message {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	out := make([]*Message, 0)
	for _, m := range ac.messages {
		include := true
		for _, pred := range predicates {
			if !pred(m) {
				include = false
				break
			}
		}
		if include {
			out = append(out, m)
		}
	}
	return out
}

// SystemPrompt returns the system prompt message, or nil if not set.
func (ac *AgentContext) SystemPrompt() *Message {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.systemPrompt
}

// TokenCount returns the cached aggregate token count in O(1).
func (ac *AgentContext) TokenCount() TokenCount {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.cachedTokens
}

// Stats returns a rich immutable snapshot of all context metrics.
// The snapshot is safe to inspect from any goroutine after it is returned.
func (ac *AgentContext) Stats() ContextStats {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return buildStats(ac.messages, ac.contextWindow, ac.cachedTokens, ac.compact, ac.tput)
}

// IsOverLimit reports whether the cached token count exceeds the window budget.
func (ac *AgentContext) IsOverLimit() bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.cachedTokens.ExceedsWindow(ac.contextWindow)
}

// Pressure returns the current WindowPressure level without computing the full Stats snapshot.
func (ac *AgentContext) Pressure() WindowPressure {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	pct := ac.contextWindow.UsagePct(ac.cachedTokens)
	return windowPressureFor(pct, ac.cachedTokens.ExceedsWindow(ac.contextWindow))
}

// Window returns the configured ContextWindow.
func (ac *AgentContext) Window() ContextWindow { return ac.contextWindow }

// TC returns the TokenComputer used by this context.
// Expose this so callers constructing new Messages can use the same instance.
func (ac *AgentContext) TC() TokenComputer { return ac.tc }

// IDGen returns the IDGenerator used by this context.
// Use this when constructing new Messages outside the context so all IDs
// share the same generation strategy.
func (ac *AgentContext) IDGen() IDGenerator { return ac.idGen }
