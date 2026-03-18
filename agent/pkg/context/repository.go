package llmctx

// R9: This package is named "llmctx" (not "context") so that files importing
// both this package and the stdlib "context" package never need awkward aliases.
// The stdlib context is imported as-is; this package is imported as "llmctx".

import gocontext "context" // alias makes the intent explicit

// ---------------------------------------------------------------------------
// MessageFilter
// ---------------------------------------------------------------------------

// MessageFilter carries optional query predicates for listing messages.
// Zero-value fields are ignored (no constraint applied).
type MessageFilter struct {
	// AuthorID restricts results to a specific author when non-zero.
	AuthorID AuthorID

	// Types restricts results to the listed MessageTypes when non-empty.
	Types []MessageType

	// WeightAtLeast restricts results to messages with weight >= this value
	// when non-nil.
	WeightAtLeast *MessageWeight

	// Limit caps the number of returned messages (0 = unlimited).
	Limit int

	// Offset skips the first N matching messages (for pagination).
	Offset int
}

// ---------------------------------------------------------------------------
// SaveResult
// ---------------------------------------------------------------------------

// SaveResult is returned by MessageRepository.Save to describe what the store did.
type SaveResult struct {
	// Created is true when the message was inserted; false when updated.
	Created bool

	// StoredAt is the timestamp recorded by the backing store.
	StoredAt Timestamp
}

// ---------------------------------------------------------------------------
// MessageRepository
// ---------------------------------------------------------------------------

// MessageRepository is the persistence port for Message entities.
// Implementations must be safe for concurrent use.
// All methods accept a gocontext.Context for deadline, cancellation, and tracing.
type MessageRepository interface {
	// Save persists a message. If a message with the same ID already exists
	// the store upserts it. Returns SaveResult describing the outcome.
	Save(ctx gocontext.Context, message *Message) (SaveResult, error)

	// SaveBatch persists multiple messages in a single round-trip where
	// possible. Must guarantee all-or-nothing (atomic) semantics.
	SaveBatch(ctx gocontext.Context, messages []*Message) error

	// FindByID retrieves a single message by its ID.
	// Returns *ContextError with ErrCodeMessageNotFound when absent.
	FindByID(ctx gocontext.Context, id MessageID) (*Message, error)

	// List retrieves messages matching the filter, ordered by CreatedAt ascending.
	List(ctx gocontext.Context, filter MessageFilter) ([]*Message, error)

	// DeleteByID removes the message with the given ID.
	// Returns *ContextError with ErrCodeMessageNotFound when absent.
	DeleteByID(ctx gocontext.Context, id MessageID) error

	// DeleteByWeight removes all messages matching weight.
	// Returns the count of deleted messages.
	DeleteByWeight(ctx gocontext.Context, weight MessageWeight) (int, error)

	// CountTokens returns the aggregate TokenCount of all matching messages
	// without loading message bodies. Implementations should push this query
	// to the backing store rather than fetching all rows.
	CountTokens(ctx gocontext.Context, filter MessageFilter) (TokenCount, error)

	// Clear removes every message in the store.
	// Must be guarded by appropriate access control at the call site.
	Clear(ctx gocontext.Context) error
}

// ---------------------------------------------------------------------------
// ContextSnapshot
// ---------------------------------------------------------------------------

// ContextSnapshot is a serialisable point-in-time capture of an AgentContext.
type ContextSnapshot struct {
	SnapshotID MessageID     `json:"snapshot_id"`
	CapturedAt Timestamp     `json:"captured_at"`
	Window     ContextWindow `json:"window"`
	Messages   []*Message    `json:"messages"`
	Stats      ContextStats  `json:"stats"`
}

// ---------------------------------------------------------------------------
// ContextSnapshotRepository
// ---------------------------------------------------------------------------

// ContextSnapshotRepository persists and restores full AgentContext snapshots.
// This is a higher-level abstraction layered on MessageRepository when an
// entire context needs to be checkpointed (e.g. for session resumption).
type ContextSnapshotRepository interface {
	// SaveSnapshot persists a complete snapshot.
	SaveSnapshot(ctx gocontext.Context, snapshot *ContextSnapshot) error

	// LoadSnapshot retrieves the snapshot with the given ID.
	// Returns *ContextError with ErrCodeMessageNotFound when absent.
	LoadSnapshot(ctx gocontext.Context, id MessageID) (*ContextSnapshot, error)

	// ListSnapshots returns metadata (without message bodies) for all
	// snapshots, ordered by CapturedAt descending.
	ListSnapshots(ctx gocontext.Context) ([]*ContextSnapshot, error)

	// DeleteSnapshot permanently removes a snapshot.
	DeleteSnapshot(ctx gocontext.Context, id MessageID) error
}
