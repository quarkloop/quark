package llmctx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToFlatString serialises all messages to a human-readable flat string.
// Format per line:  [<kind>] <author>: <content>
func ToFlatString(ac *AgentContext) string {
	messages := ac.Messages()
	var sb strings.Builder
	for i, m := range messages {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "[%s] %s: %s", m.Type(), m.Author(), m.Content().String())
	}
	return sb.String()
}

// ToUserFlatString serialises only user-visible messages to a flat string.
func ToUserFlatString(ac *AgentContext) string {
	messages := ac.VisibleMessages(VisibleToUser)
	var sb strings.Builder
	for i, m := range messages {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "[%s] %s: %s", m.Type(), m.Author(), m.Content().String())
	}
	return sb.String()
}

// ToJSON serialises all messages to a JSON array.
func ToJSON(ac *AgentContext) ([]byte, error) {
	data, err := json.Marshal(ac.Messages())
	if err != nil {
		return nil, newErr(ErrCodeSerializationFailed, "failed to marshal context to JSON", err)
	}
	return data, nil
}

// ToStatsJSON serialises the full ContextStats snapshot to JSON.
func ToStatsJSON(ac *AgentContext) ([]byte, error) {
	data, err := json.Marshal(ac.Stats())
	if err != nil {
		return nil, newErr(ErrCodeSerializationFailed, "failed to marshal stats to JSON", err)
	}
	return data, nil
}

// SnapshotFromContext captures the current state as a ContextSnapshot.
func SnapshotFromContext(snapshotID MessageID, ac *AgentContext) *ContextSnapshot {
	return &ContextSnapshot{
		SnapshotID: snapshotID,
		CapturedAt: Now(),
		Window:     ac.Window(),
		Messages:   ac.Messages(),
		Stats:      ac.Stats(),
	}
}

// =============================================================================
// R17: Round-trip deserialisation
// =============================================================================

// ContextFromSnapshot restores an AgentContext from a previously captured
// ContextSnapshot. The snapshot is typically produced by SnapshotFromContext
// and persisted via a ContextSnapshotRepository.
//
// Behaviour:
//   - The system prompt (if present) is identified by MessageType and re-set
//     as the protected system prompt in the rebuilt context.
//   - Token counts are taken from the snapshot as-is (no recomputation).
//     Pass a TokenComputer in opts to recompute counts from the current model.
//   - The compactor and IDGenerator are not persisted in a snapshot; supply
//     them via opts if needed.
//
// Returns *ContextError with ErrCodeInvalidConfig when tc is nil.
func ContextFromSnapshot(snapshot *ContextSnapshot, tc TokenComputer, opts ...RestoreOption) (*AgentContext, error) {
	if tc == nil {
		return nil, newErr(ErrCodeInvalidConfig,
			"ContextFromSnapshot: TokenComputer must not be nil", nil)
	}
	cfg := defaultRestoreConfig()
	for _, o := range opts {
		o(&cfg)
	}

	b := NewAgentContextBuilder().
		WithContextWindow(snapshot.Window).
		WithTokenComputer(tc)

	if cfg.compactor != nil {
		b = b.WithCompactor(cfg.compactor)
	}
	if cfg.idGen != nil {
		b = b.WithIDGenerator(cfg.idGen)
	}

	// Identify the system prompt before building so WithSystemPrompt is set.
	for _, m := range snapshot.Messages {
		if m.Type() == SystemPromptType {
			b = b.WithSystemPrompt(m)
			break
		}
	}

	ac, err := b.Build()
	if err != nil {
		return nil, err
	}

	// Re-append all messages (Build already added the system prompt if present).
	sysID := ""
	if ac.SystemPrompt() != nil {
		sysID = ac.SystemPrompt().ID().String()
	}
	for _, m := range snapshot.Messages {
		if m.ID().String() == sysID {
			continue // already in context via WithSystemPrompt
		}
		msg := m
		if cfg.recomputeTokens {
			newTok, terr := tc.Compute(NewMessageContent(m.payload.TextRepresentation()))
			if terr == nil {
				msg = m.withTokenCount(newTok)
			}
		}
		if err := ac.AppendMessage(context.Background(), msg); err != nil {
			return nil, newErr(ErrCodeSerializationFailed,
				"ContextFromSnapshot: failed to restore message "+m.ID().String(), err)
		}
	}
	return ac, nil
}

// ContextFromJSON deserialises a JSON byte slice previously produced by
// ToJSON into a fresh AgentContext.
//
// The JSON must be an array of Message objects as produced by MarshalJSON.
// Metadata like ContextWindow and Stats are not embedded in this format; use
// ContextFromSnapshot for full round-trip fidelity.
func ContextFromJSON(data []byte, tc TokenComputer, opts ...RestoreOption) (*AgentContext, error) {
	if tc == nil {
		return nil, newErr(ErrCodeInvalidConfig,
			"ContextFromJSON: TokenComputer must not be nil", nil)
	}

	var messages []*Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, newErr(ErrCodeSerializationFailed,
			"ContextFromJSON: failed to unmarshal message array", err)
	}

	// Synthesise a minimal snapshot so we can reuse ContextFromSnapshot.
	snap := &ContextSnapshot{
		SnapshotID: MustMessageID("restored"),
		CapturedAt: Now(),
		Messages:   messages,
	}
	return ContextFromSnapshot(snap, tc, opts...)
}

// =============================================================================
// RestoreOption — functional options for restoration
// =============================================================================

// RestoreOption is a functional option for ContextFromSnapshot and ContextFromJSON.
type RestoreOption func(*restoreConfig)

type restoreConfig struct {
	compactor       Compactor
	idGen           IDGenerator
	recomputeTokens bool
}

func defaultRestoreConfig() restoreConfig {
	return restoreConfig{}
}

// WithRestoreCompactor injects a Compactor into the restored AgentContext.
func WithRestoreCompactor(c Compactor) RestoreOption {
	return func(cfg *restoreConfig) { cfg.compactor = c }
}

// WithRestoreIDGenerator injects an IDGenerator into the restored AgentContext.
func WithRestoreIDGenerator(g IDGenerator) RestoreOption {
	return func(cfg *restoreConfig) { cfg.idGen = g }
}

// WithRecomputeTokens instructs the restore functions to recompute token
// counts using the supplied TokenComputer rather than trusting stored values.
// Useful when the tokeniser has changed between sessions.
func WithRecomputeTokens() RestoreOption {
	return func(cfg *restoreConfig) { cfg.recomputeTokens = true }
}
