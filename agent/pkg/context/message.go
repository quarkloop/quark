package llmctx

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/quarkloop/agent/pkg/context/freshness"
	"github.com/quarkloop/agent/pkg/context/linking"
	msg "github.com/quarkloop/agent/pkg/context/message"
)

// =============================================================================
// message.go
//
// Defines:
//   - TokenComputer interface (implemented in tokenizer.go)
//   - Compactor interface    (implemented in compactor.go)
//   - Message struct and all methods
//   - Type-specific New*Message factory functions
// =============================================================================

// -----------------------------------------------------------------------------
// Message
// -----------------------------------------------------------------------------

// Message is the fundamental, immutable-by-default unit of an AgentContext.
// It wraps a typed msg.Payload and carries metadata: weight, visibility, timestamps,
// structural links, freshness policy, and importance decay function.
//
// Fields are unexported; all access is through methods. Use the typed
// New*Message factories to construct messages.
type Message struct {
	id         MessageID
	authorId   AuthorID
	author     MessageAuthor
	weight     MessageWeight
	vis        Visibility
	payload    msg.Payload
	tokenCount TokenCount
	createdAt  Timestamp
	updatedAt  *Timestamp

	// links carries structural relationships to other messages.
	// Zero value means no structural links.
	links linking.MessageLinks

	// freshnessPolicy controls staleness detection for this message.
	// nil means the message is always considered fresh.
	freshnessPolicy freshness.FreshnessPolicy

	// decayFn modulates the message's effective weight during compaction.
	// nil means no decay (equivalent to freshness.NoDecay()).
	decayFn freshness.DecayFunction
}

// messageWire is the JSON wire shape for Message serialisation.
// The payload is encoded with its Kind so it can be identified on decode.
type messageWire struct {
	ID         MessageID     `json:"id"`
	AuthorID   AuthorID      `json:"author_id"`
	Author     MessageAuthor `json:"author"`
	Weight     MessageWeight `json:"weight"`
	Visibility Visibility    `json:"visibility"`
	Kind       MessageType   `json:"kind"`
	Payload     msg.Payload   `json:"payload"`
	TokenCount TokenCount    `json:"token_count"`
	CreatedAt  Timestamp     `json:"created_at"`
	UpdatedAt  *Timestamp    `json:"updated_at,omitempty"`
}

// MarshalJSON encodes the Message with a {kind, payload} typed envelope so the
// payload type is unambiguous in JSON.
func (m *Message) MarshalJSON() ([]byte, error) {
	return json.Marshal(messageWire{
		ID:         m.id,
		AuthorID:   m.authorId,
		Author:     m.author,
		Weight:     m.weight,
		Visibility: m.vis,
		Kind:       m.payload.Kind(),
		Payload:    m.payload,
		TokenCount: m.tokenCount,
		CreatedAt:  m.createdAt,
		UpdatedAt:  m.updatedAt,
	})
}

// UnmarshalJSON decodes a Message that was previously encoded by MarshalJSON.
//
// The wire format carries a "kind" field that is used to dispatch to the
// correct concrete Payload type before unmarshalling the "payload" object.
// Every MessageType constant that exists in this package is supported.
//
// Token counts are restored from the stored value; no TokenComputer is required
// during deserialisation. If you want counts to be recomputed from the current
// tokeniser, use ContextFromSnapshot with a live TokenComputer instead.
func (m *Message) UnmarshalJSON(b []byte) error {
	// First pass: decode the envelope to extract Kind and the raw payload bytes.
	var envelope struct {
		ID         MessageID       `json:"id"`
		AuthorID   AuthorID        `json:"author_id"`
		Author     MessageAuthor   `json:"author"`
		Weight     MessageWeight   `json:"weight"`
		Visibility Visibility      `json:"visibility"`
		Kind       MessageType     `json:"kind"`
		Payload     json.RawMessage `json:"payload"`
		TokenCount TokenCount      `json:"token_count"`
		CreatedAt  Timestamp       `json:"created_at"`
		UpdatedAt  *Timestamp      `json:"updated_at,omitempty"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return newErr(ErrCodeSerializationFailed, "Message.UnmarshalJSON: failed to decode envelope", err)
	}

	// Second pass: dispatch on Kind to decode the concrete Payload type.
	payload, err := unmarshalPayload(envelope.Kind, envelope.Payload)
	if err != nil {
		return err
	}

	m.id = envelope.ID
	m.authorId = envelope.AuthorID
	m.author = envelope.Author
	m.weight = envelope.Weight
	m.vis = envelope.Visibility
	m.payload = payload
	m.tokenCount = envelope.TokenCount
	m.createdAt = envelope.CreatedAt
	m.updatedAt = envelope.UpdatedAt
	return nil
}

// unmarshalPayload decodes a raw JSON object into the concrete Payload type
// identified by kind. Returns an error for unrecognised kinds.
//
// All payloads are returned as value types (not pointers) to match the storage
// convention used by New*Message factories — which is what As*() type assertions
// expect.
func unmarshalPayload(kind MessageType, raw json.RawMessage) (msg.Payload, error) {
	// decode decodes raw JSON into dst and returns the dereferenced value.
	// Using a pointer target for json.Unmarshal is idiomatic; we immediately
	// dereference so the interface value holds the value type, not *T.
	decode := func(dst msg.Payload, out *msg.Payload) error {
		if err := json.Unmarshal(raw, dst); err != nil {
			return newErr(ErrCodeSerializationFailed,
				fmt.Sprintf("unmarshalPayload: failed to decode %q payload", kind), err)
		}
		*out = dst
		return nil
	}

	var p msg.Payload
	var err error
	switch kind {
	case SystemPromptType:
		var v msg.SystemPromptPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case TextMessageType:
		var v msg.TextPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case ImageMessageType:
		var v msg.ImagePayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case PDFMessageType:
		var v msg.PDFPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case AudioMessageType:
		var v msg.AudioPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case ToolCallMessageType:
		var v msg.ToolCallPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case ToolResultMessageType:
		var v msg.ToolResultPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case MemoryMessageType:
		var v msg.MemoryPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case ReasoningMessageType:
		var v msg.ReasoningPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case LogMessageType:
		var v msg.LogPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case ErrorMessageType:
		var v msg.ErrorPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	case PlanMessageType:
		var v msg.PlanPayload
		err = decode(&v, &p)
		if err == nil {
			p = v
		}
	default:
		return nil, newErr(ErrCodeSerializationFailed,
			fmt.Sprintf("unmarshalPayload: unknown message kind %q", kind), nil)
	}
	return p, err
}

// -----------------------------------------------------------------------------
// Internal constructor
// -----------------------------------------------------------------------------

// newMessage is the single internal path for creating a Message.
// All New*Message factories call this; callers should not use it directly.
func newMessage(
	id MessageID,
	authorId AuthorID,
	author MessageAuthor,
	weight MessageWeight,
	vis Visibility,
	payload msg.Payload,
	tc TokenComputer,
) (*Message, error) {
	if tc == nil {
		return nil, newErr(ErrCodeInvalidMessage, "TokenComputer must not be nil", nil)
	}
	tokens, err := tc.Compute(NewMessageContent(payload.TextRepresentation()))
	if err != nil {
		return nil, newErr(ErrCodeTokenComputeFailed,
			fmt.Sprintf("failed to compute tokens for message %s", id), err)
	}
	return &Message{
		id:         id,
		authorId:   authorId,
		author:     author,
		weight:     weight,
		vis:        vis,
		payload:    payload,
		tokenCount: tokens,
		createdAt:  Now(),
	}, nil
}

// -----------------------------------------------------------------------------
// Typed factory functions
// -----------------------------------------------------------------------------
// Each factory:
//   - Picks the canonical author, weight, and default visibility for the type.
//   - Accepts only the payload fields that make sense for that type.
//   - Returns (*Message, error) so callers handle construction failures.

// NewSystemPromptMessage creates the agent's top-level system instructions.
// Uses HighestWeight so the system prompt survives all compaction strategies.
func NewSystemPromptMessage(
	id MessageID, authorId AuthorID,
	text string, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, SystemAuthor, HighestWeight,
		defaultVisibility[SystemPromptType],
		msg.SystemPromptPayload{Text: text}, tc)
}

// NewTextMessage creates a plain-text conversational turn.
func NewTextMessage(
	id MessageID, authorId AuthorID, author MessageAuthor,
	text string, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, author, MediumWeight,
		defaultVisibility[TextMessageType],
		msg.TextPayload{Text: text}, tc)
}

// NewImageMessage creates an image message for vision-capable agents.
func NewImageMessage(
	id MessageID, authorId AuthorID, author MessageAuthor,
	payload msg.ImagePayload, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, author, MediumWeight,
		defaultVisibility[ImageMessageType], payload, tc)
}

// NewPDFMessage creates a PDF document message for document-QA or RAG workflows.
func NewPDFMessage(
	id MessageID, authorId AuthorID, author MessageAuthor,
	payload msg.PDFPayload, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, author, MediumWeight,
		defaultVisibility[PDFMessageType], payload, tc)
}

// NewAudioMessage creates an audio message for speech-enabled agents.
func NewAudioMessage(
	id MessageID, authorId AuthorID, author MessageAuthor,
	payload msg.AudioPayload, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, author, MediumWeight,
		defaultVisibility[AudioMessageType], payload, tc)
}

// NewToolCallMessage records the LLM's decision to invoke a tool.
// Author is always AgentAuthor because the LLM made the decision.
func NewToolCallMessage(
	id MessageID, authorId AuthorID,
	payload msg.ToolCallPayload, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, AgentAuthor, MediumWeight,
		defaultVisibility[ToolCallMessageType], payload, tc)
}

// NewToolResultMessage records the output returned by a tool execution.
// Author is always ToolAuthor.
func NewToolResultMessage(
	id MessageID, authorId AuthorID,
	payload msg.ToolResultPayload, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, ToolAuthor, MediumWeight,
		defaultVisibility[ToolResultMessageType], payload, tc)
}

// NewMemoryMessage injects a memory entry into the context.
// Uses LowestWeight so memories are the first to be evicted during compaction.
func NewMemoryMessage(
	id MessageID, authorId AuthorID,
	payload msg.MemoryPayload, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, SystemAuthor, LowestWeight,
		defaultVisibility[MemoryMessageType], payload, tc)
}

// NewReasoningMessage records a chain-of-thought or scratchpad step.
// Uses LowestWeight; dev-only visibility by default.
func NewReasoningMessage(
	id MessageID, authorId AuthorID,
	payload msg.ReasoningPayload, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, AgentAuthor, LowestWeight,
		defaultVisibility[ReasoningMessageType], payload, tc)
}

// NewLogMessage stores a structured audit log entry in the context history.
// Log messages are never forwarded to the LLM or surfaced to users.
func NewLogMessage(
	id MessageID, authorId AuthorID,
	payload msg.LogPayload, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, SystemAuthor, LowestWeight,
		defaultVisibility[LogMessageType], payload, tc)
}

// NewErrorMessage records a structured error event in the context.
// Visible to users and developers; allows error-aware reasoning.
func NewErrorMessage(
	id MessageID, authorId AuthorID,
	payload msg.ErrorPayload, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, SystemAuthor, MediumWeight,
		defaultVisibility[ErrorMessageType], payload, tc)
}

// NewPlanMessage records a structured multi-step execution plan.
// Uses HighestWeight so plans survive compaction alongside the system prompt.
func NewPlanMessage(
	id MessageID, authorId AuthorID,
	payload msg.PlanPayload, tc TokenComputer,
) (*Message, error) {
	return newMessage(id, authorId, AgentAuthor, HighestWeight,
		defaultVisibility[PlanMessageType], payload, tc)
}

// -----------------------------------------------------------------------------
// Read-only accessors
// -----------------------------------------------------------------------------

func (m *Message) ID() MessageID          { return m.id }
func (m *Message) AuthorID() AuthorID     { return m.authorId }
func (m *Message) Author() MessageAuthor  { return m.author }
func (m *Message) Type() MessageType      { return m.payload.Kind() }
func (m *Message) Weight() MessageWeight  { return m.weight }
func (m *Message) Visibility() Visibility { return m.vis }
func (m *Message) Payload() msg.Payload       { return m.payload }
func (m *Message) TokenCount() TokenCount { return m.tokenCount }
func (m *Message) CreatedAt() Timestamp   { return m.createdAt }
func (m *Message) UpdatedAt() *Timestamp  { return m.updatedAt }

// Content returns the flat text representation of the payload.
// Used by tokenisers and flat-string serialisers.
func (m *Message) Content() MessageContent {
	return NewMessageContent(m.payload.TextRepresentation())
}

// LLMContent returns the string that adapters inject into the LLM context.
// For most types this equals Content().String(). msg.LogPayload returns "".
func (m *Message) LLMContent() string { return m.payload.LLMText() }

// IsVisibleTo reports whether this message is surfaced to the given target.
func (m *Message) IsVisibleTo(target Visibility) bool { return m.vis.HasFlag(target) }

// -----------------------------------------------------------------------------
// Non-mutating copy helpers
// -----------------------------------------------------------------------------

// WithVisibility returns a shallow copy of the message with visibility set to v.
// Use this to override the default after construction without modifying the original.
func (m *Message) WithVisibility(v Visibility) *Message {
	c := *m
	c.vis = v
	return &c
}

// WithWeight returns a shallow copy of the message with weight set to w.
func (m *Message) WithWeight(w MessageWeight) *Message {
	c := *m
	c.weight = w
	return &c
}

// WithLinks returns a shallow copy of the message with the given structural links.
//
// Links express containment and reference relationships between messages.
// The compactor uses them to evict messages as coherent units (e.g. a
// ToolCall and its ToolResult are never split).
//
// Example — mark a ToolResult as a child of its ToolCall:
//
//	result = result.WithLinks(
//	    linking.MessageLinks{}.WithParent(toolCallMsg.ID().String()),
//	)
func (m *Message) WithLinks(l linking.MessageLinks) *Message {
	c := *m
	c.links = l
	return &c
}

// Links returns the structural relationships attached to this message.
// Zero value when no links have been set.
func (m *Message) Links() linking.MessageLinks { return m.links }

// WithFreshnessPolicy attaches a staleness policy to the message.
//
// The policy is consulted by AgentContext.ScanFreshness before each LLM
// request.  When IsStale returns true the scanner either replaces the
// message content (if the policy implements Refresh) or flags it.
//
// Example — message is stale after 5 minutes:
//
//	msg = msg.WithFreshnessPolicy(freshness.NewTTLPolicy(5 * time.Minute))
//
// Example — contract dates are always fresh:
//
//	msg = msg.WithFreshnessPolicy(freshness.ImmutableFreshnessPolicy{})
func (m *Message) WithFreshnessPolicy(p freshness.FreshnessPolicy) *Message {
	c := *m
	c.freshnessPolicy = p
	return &c
}

// FreshnessPolicy returns the policy attached to this message, or nil when none.
func (m *Message) FreshnessPolicy() freshness.FreshnessPolicy { return m.freshnessPolicy }

// WithDecayFn attaches a decay function that modulates the message's effective
// importance during compaction.  A return value of 1.0 means full importance
// (no decay); 0.0 means fully decayed (evict this first).
//
// Example — importance halves every 10 minutes:
//
//	msg = msg.WithDecayFn(freshness.ExponentialDecay(10 * time.Minute))
func (m *Message) WithDecayFn(fn freshness.DecayFunction) *Message {
	c := *m
	c.decayFn = fn
	return &c
}

// DecayFn returns the decay function attached to this message, or nil when none.
func (m *Message) DecayFn() freshness.DecayFunction { return m.decayFn }

// EffectiveWeight returns the message's weight multiplied by its decay score.
//
// Parameters:
//   - position: the message's current index in the context slice.
//   - total:    the total number of messages in the context.
//   - now:      current wall-clock time used to compute age.
//
// When no decay function is set, returns the raw weight value unchanged.
func (m *Message) EffectiveWeight(position, total int, now time.Time) float64 {
	base := float64(m.weight.Value())
	if m.decayFn == nil {
		return base
	}
	result := freshness.EvaluateDecay(m.decayFn, m.createdAt.Time(), position, total, now)
	return base * result.Score
}

// IDString returns the string representation of the message ID.
// This method satisfies the llmctx/compactor.Message interface without
// requiring the compactor package to import the root package.
func (m *Message) IDString() string { return m.id.value }

// AuthorString returns the string representation of the message author role.
// This method satisfies the llmctx/compactor.Message interface.
func (m *Message) AuthorString() string { return string(m.author) }

// WeightValue returns the raw int32 weight value.
// This method satisfies the llmctx/compactor.Message interface.
func (m *Message) WeightValue() int32 { return m.weight.value }

// withTokenCount returns a shallow copy of the message with the token count
// replaced. Used during snapshot restoration when recomputeTokens is set.
func (m *Message) withTokenCount(t TokenCount) *Message {
	c := *m
	c.tokenCount = t
	return &c
}

// MessageID implements linking.LinkedMessage — returns the message ID as a string.
// This allows *Message to satisfy the graph interface without importing the parent package.
func (m *Message) MessageID() string { return m.id.value }

// -----------------------------------------------------------------------------
// Mutating update
// -----------------------------------------------------------------------------

// SetPayload replaces the payload and recomputes the token count.
// Returns the previous TokenCount so AgentContext can update its running total.
// This is the only mutating method on Message.
func (m *Message) SetPayload(payload msg.Payload, tc TokenComputer) (oldTokens TokenCount, err error) {
	if tc == nil {
		return TokenCount{}, newErr(ErrCodeInvalidMessage, "TokenComputer must not be nil", nil)
	}
	tokens, err := tc.Compute(NewMessageContent(payload.TextRepresentation()))
	if err != nil {
		return TokenCount{}, newErr(ErrCodeTokenComputeFailed,
			fmt.Sprintf("failed to recompute tokens for message %s", m.id), err)
	}
	old := m.tokenCount
	m.payload = payload
	m.tokenCount = tokens
	t := Now()
	m.updatedAt = &t
	return old, nil
}

// -----------------------------------------------------------------------------
// msg.Payload type-assertion helpers
// -----------------------------------------------------------------------------
// Each helper returns (payload, true) if the message carries that type,
// or (zero, false) otherwise. Prefer these over direct type switches.

func (m *Message) AsSystemPrompt() (msg.SystemPromptPayload, bool) {
	p, ok := m.payload.(msg.SystemPromptPayload)
	return p, ok
}
func (m *Message) AsText() (msg.TextPayload, bool) {
	p, ok := m.payload.(msg.TextPayload)
	return p, ok
}
func (m *Message) AsImage() (msg.ImagePayload, bool) {
	p, ok := m.payload.(msg.ImagePayload)
	return p, ok
}
func (m *Message) AsPDF() (msg.PDFPayload, bool) {
	p, ok := m.payload.(msg.PDFPayload)
	return p, ok
}
func (m *Message) AsAudio() (msg.AudioPayload, bool) {
	p, ok := m.payload.(msg.AudioPayload)
	return p, ok
}
func (m *Message) AsToolCall() (msg.ToolCallPayload, bool) {
	p, ok := m.payload.(msg.ToolCallPayload)
	return p, ok
}
func (m *Message) AsToolResult() (msg.ToolResultPayload, bool) {
	p, ok := m.payload.(msg.ToolResultPayload)
	return p, ok
}
func (m *Message) AsMemory() (msg.MemoryPayload, bool) {
	p, ok := m.payload.(msg.MemoryPayload)
	return p, ok
}
func (m *Message) AsReasoning() (msg.ReasoningPayload, bool) {
	p, ok := m.payload.(msg.ReasoningPayload)
	return p, ok
}
func (m *Message) AsLog() (msg.LogPayload, bool) {
	p, ok := m.payload.(msg.LogPayload)
	return p, ok
}
func (m *Message) AsError() (msg.ErrorPayload, bool) {
	p, ok := m.payload.(msg.ErrorPayload)
	return p, ok
}
func (m *Message) AsPlan() (msg.PlanPayload, bool) {
	p, ok := m.payload.(msg.PlanPayload)
	return p, ok
}
