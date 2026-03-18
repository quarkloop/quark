package llmctx

// =============================================================================
// interfaces.go  —  ContextReader and ContextWriter interfaces
//
// M8: Defines the two canonical interface contracts for AgentContext.
//
// Motivation:
//   - Code that only inspects context state should accept ContextReader, not
//     *AgentContext, so it can be tested with any implementation.
//   - Code that mutates context (appends, removes, compacts) should accept
//     ContextWriter.
//   - *AgentContext satisfies both interfaces.
//   - Future implementations (e.g. a distributed context, a read-only view,
//     a test double) can satisfy one or both without inheriting the full type.
//
// Contract notes:
//   - ContextReader methods are safe for concurrent use (matches *AgentContext).
//   - ContextWriter mutation methods accept context.Context for cancellation.
//   - Context (the Go stdlib type) is intentionally distinct from the llmctx
//     notion of "context" — the parameter name is always "ctx".
// =============================================================================

import (
	"context"

	msg "github.com/quarkloop/agent/pkg/context/message"
)

// =============================================================================
// ContextReader — read-only view of an agent context
// =============================================================================

// ContextReader exposes the read-only surface of an AgentContext.
//
// All methods must be safe for concurrent use from multiple goroutines.
// *AgentContext satisfies this interface.
type ContextReader interface {
	// FindMessage returns the message with the given ID, or an error if absent.
	FindMessage(id MessageID) (*Message, error)

	// Messages returns all messages in insertion order, including the system
	// prompt if one is set.
	Messages() []*Message

	// SystemPrompt returns the pinned system-prompt message, or nil if none.
	SystemPrompt() *Message

	// LLMMessages returns only messages that are visible to the LLM (i.e. those
	// whose Visibility includes VisibleToLLM). This is the slice passed to the
	// LLM provider on every inference call.
	LLMMessages() []*Message

	// VisibleMessages returns messages visible on the given surface.
	VisibleMessages(target Visibility) []*Message

	// FilterMessages returns the subset of messages that satisfy all predicates.
	FilterMessages(predicates ...func(*Message) bool) []*Message

	// TokenCount returns the current aggregate token count (O(1), cached).
	TokenCount() TokenCount

	// Window returns the configured context window budget.
	Window() ContextWindow

	// IsOverLimit reports whether the current token count exceeds the window.
	IsOverLimit() bool

	// Pressure returns the current window pressure level.
	Pressure() WindowPressure

	// Stats returns an immutable snapshot of context metrics.
	Stats() ContextStats

	// TC returns the TokenComputer used by this context.
	TC() TokenComputer

	// IDGen returns the IDGenerator used by this context.
	IDGen() IDGenerator
}

// =============================================================================
// ContextWriter — mutation surface of an agent context
// =============================================================================

// ContextWriter exposes the mutation surface of an AgentContext.
//
// All mutation methods accept a context.Context as their first argument for
// cancellation and deadline propagation.
// *AgentContext satisfies this interface.
type ContextWriter interface {
	// AppendMessage adds a message to the end of the context.
	//
	// Returns an error if ctx is cancelled, the message is nil, or write-time
	// middleware fails. See AgentContext.AppendMessage for full semantics.
	AppendMessage(ctx context.Context, message *Message) error

	// RemoveMessageByID removes the message with the given ID.
	//
	// Returns ErrCodeSystemPromptLocked when the system prompt is targeted.
	// Returns ErrCodeMessageNotFound when the ID is absent.
	RemoveMessageByID(ctx context.Context, id MessageID) error

	// RemoveMessagesByWeight removes all non-system messages whose weight
	// equals weight and returns the count removed.
	RemoveMessagesByWeight(ctx context.Context, weight MessageWeight) int

	// UpdateMessagePayload replaces the payload on the message with the given ID.
	//
	// Returns ErrCodeMessageNotFound when the ID is absent.
	UpdateMessagePayload(ctx context.Context, id MessageID, payload msg.Payload) error

	// Compact runs the configured Compactor to bring the context within its
	// window budget. No-ops when the window is unbounded or already satisfied.
	Compact(ctx context.Context) error

	// ApplyPolicy bulk-applies a VisibilityPolicy to all current messages.
	// The policy is not stored on the context — it applies to current messages only.
	ApplyPolicy(policy *VisibilityPolicy)
}

// =============================================================================
// Context — convenience interface combining reader + writer
// =============================================================================

// Context is the full AgentContext contract: both read and write access.
// *AgentContext satisfies this interface.
//
// Prefer accepting ContextReader or ContextWriter at call sites when only
// one surface is needed — this keeps dependencies minimal and enables
// read-only views and test doubles.
type Context interface {
	ContextReader
	ContextWriter
}

// Verify that *AgentContext satisfies all three interfaces at compile time.
var (
	_ ContextReader = (*AgentContext)(nil)
	_ ContextWriter = (*AgentContext)(nil)
	_ Context       = (*AgentContext)(nil)
)
