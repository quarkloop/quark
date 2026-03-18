package llmctx

import (
	"github.com/quarkloop/agent/pkg/context/contradiction"
	"github.com/quarkloop/agent/pkg/context/semantic"
)

// =============================================================================
// middleware.go  —  Write-time middleware for AppendMessage
//
// Concept:
//   AgentContext supports opt-in middleware that runs synchronously inside
//   AppendMessage before the message is added to the slice.  Middleware can:
//     - Detect and optionally merge semantically similar messages (semantic compression)
//     - Detect and flag contradictory messages (contradiction detection)
//
//   Both are opt-in.  When no middleware is configured AppendMessage has the
//   same performance characteristics as before.
//
// Usage:
//   // Semantic compression
//   idx := semantic.NewSemanticIndex(nil)
//   compressor, _ := semantic.NewSemanticCompressor(idx, 0.75)
//   compressor.WithAutoMerge(0.90, func(existing, incoming string) (string, error) {
//       return existing + "\n" + incoming, nil  // or call an LLM
//   })
//   ac.UseSemanticCompressor(compressor)
//
//   // Contradiction detection
//   detector := contradiction.NewHeuristicDetector()
//   ac.UseContradictionDetector(detector)
//
//   // Both can be active simultaneously.
// =============================================================================

// UseSemanticCompressor attaches a SemanticCompressor as write-time middleware.
//
// When set, every AppendMessage call runs the compressor's Check method before
// adding the message.  If an auto-merge fires, the existing message is updated
// in-place and the incoming message is NOT appended.
//
// Calling this method replaces any previously configured compressor.
// Pass nil to remove semantic compression.
func (ac *AgentContext) UseSemanticCompressor(sc *semantic.SemanticCompressor) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.semanticCompressor = sc
}

// UseContradictionDetector attaches a ContradictionDetector as write-time middleware.
//
// When set, every AppendMessage call runs the detector's Check method.
// Warnings are accumulated in the most-recent ContradictionWarnings slice,
// readable via PendingContradictions.  The message is always appended —
// the detector never blocks insertion.
//
// Calling this method replaces any previously configured detector.
// Pass nil to remove contradiction detection.
func (ac *AgentContext) UseContradictionDetector(d contradiction.ContradictionDetector) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.contradictionDetector = d
}

// PendingContradictions returns warnings accumulated since the last call to
// ClearContradictions, in detection order.
//
// The caller is responsible for deciding what to do with them: log them,
// surface them to the user, or remove the conflicting messages.
func (ac *AgentContext) PendingContradictions() []contradiction.ContradictionWarning {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	out := make([]contradiction.ContradictionWarning, len(ac.pendingContradictions))
	copy(out, ac.pendingContradictions)
	return out
}

// ClearContradictions empties the pending warnings list.
func (ac *AgentContext) ClearContradictions() {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.pendingContradictions = ac.pendingContradictions[:0]
}

// =============================================================================
// Internal middleware plumbing — called inside AppendMessage
// =============================================================================

// runSemanticMiddleware checks the incoming message against the semantic index.
//
// Returns (shouldSkipAppend bool, err).
// shouldSkipAppend is true when an auto-merge fired and the message should not
// be added to the context.
//
// Must be called with ac.mu held (write lock).
func (ac *AgentContext) runSemanticMiddleware(m *Message) (skipAppend bool, err error) {
	if ac.semanticCompressor == nil {
		return false, nil
	}
	text := m.LLMContent()
	if text == "" {
		// Non-LLM messages (logs etc.) don't participate in semantic indexing.
		return false, nil
	}

	result, checkErr := ac.semanticCompressor.Check(m.ID().String(), text)
	if checkErr != nil {
		// Middleware errors are non-fatal: log and continue.
		// Semantic compression failure must never block message appending.
		return false, nil
	}

	if result.AutoMerged != nil {
		// Find and update the merged message.
		pos, ok := ac.index[result.AutoMerged.ReplacedID]
		if ok {
			existing := ac.messages[pos]
			refreshed, buildErr := buildRefreshedPayload(existing, result.AutoMerged.MergedText)
			if buildErr == nil {
				oldTokens, setErr := existing.SetPayload(refreshed, ac.tc)
				if setErr == nil {
					ac.cachedTokens = ac.cachedTokens.Sub(oldTokens).Add(existing.tokenCount)
					// Re-index the merged message in the semantic index.
					_ = ac.semanticCompressor.Index(result.AutoMerged.ReplacedID, result.AutoMerged.MergedText)
					return true, nil // suppress appending the incoming message
				}
			}
		}
	}

	// No auto-merge: index the incoming message after it is appended.
	return false, nil
}

// runContradictionMiddleware checks the incoming message against the contradiction index.
// Must be called with ac.mu held (write lock).
func (ac *AgentContext) runContradictionMiddleware(m *Message) {
	if ac.contradictionDetector == nil {
		return
	}
	text := m.LLMContent()
	if text == "" {
		return
	}
	warnings, err := ac.contradictionDetector.Check(m.ID().String(), text)
	if err != nil || len(warnings) == 0 {
		return
	}
	ac.pendingContradictions = append(ac.pendingContradictions, warnings...)
}

// indexInMiddleware adds the message to both the semantic and contradiction
// indices after it has been successfully appended.
// Must be called with ac.mu held (write lock).
func (ac *AgentContext) indexInMiddleware(m *Message) {
	text := m.LLMContent()
	if text == "" {
		return
	}
	if ac.semanticCompressor != nil {
		_ = ac.semanticCompressor.Index(m.ID().String(), text)
	}
	if ac.contradictionDetector != nil {
		_ = ac.contradictionDetector.Index(m.ID().String(), text)
	}
}

// removeFromMiddleware removes a message from both middleware indices.
// Called from RemoveMessageByID and the compaction path.
// Must be called with ac.mu held (write lock).
func (ac *AgentContext) removeFromMiddleware(id string) {
	if ac.semanticCompressor != nil {
		ac.semanticCompressor.Remove(id)
	}
	if ac.contradictionDetector != nil {
		ac.contradictionDetector.Remove(id)
	}
}
