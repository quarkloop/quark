package llmctx

import (
	"encoding/json"
	"strings"
	"time"

	msg "github.com/quarkloop/agent/pkg/context/message"
	"github.com/quarkloop/agent/pkg/context/tokenizer"
)

// ---------------------------------------------------------------------------
// Primitive type aliases — delegated to sub-packages
// ---------------------------------------------------------------------------
//
// MessageContent, TokenCount, and ContextWindow are defined in llmctx/tokenizer
// (the foundational leaf package). TokenComputer is also defined there.
// We alias them here so callers who import only "llmctx" need no extra import.

// MessageContent holds the textual payload of a Message.
// Alias of tokenizer.MessageContent.
type MessageContent = tokenizer.MessageContent

// NewMessageContent wraps a raw string in a MessageContent.
func NewMessageContent(v string) MessageContent {
	return tokenizer.NewContent(v)
}

// TokenCount is a validated, non-negative count of LLM tokens.
// Alias of tokenizer.TokenCount.
type TokenCount = tokenizer.TokenCount

// NewTokenCount validates v and returns a TokenCount.
func NewTokenCount(v int32) (TokenCount, error) { return tokenizer.NewTokenCount(v) }

// ContextWindow is the maximum token budget for one LLM request.
// Alias of tokenizer.ContextWindow.
type ContextWindow = tokenizer.ContextWindow

// NewContextWindow validates v and returns a ContextWindow.
func NewContextWindow(v int32) (ContextWindow, error) { return tokenizer.NewContextWindow(v) }

// TokenComputer computes the approximate LLM token count for a MessageContent.
// Alias of tokenizer.TokenComputer.
type TokenComputer = tokenizer.TokenComputer

// TokenComputerStats is an immutable snapshot of CachedTokenComputer metrics.
// Alias of tokenizer.Stats.
type TokenComputerStats = tokenizer.Stats

// ---------------------------------------------------------------------------
// MessageID
// ---------------------------------------------------------------------------

// MessageID is a validated, opaque string identifier for a Message.
type MessageID struct {
	value string
}

// NewMessageID validates and wraps v in a MessageID.
func NewMessageID(v string) (MessageID, error) {
	if strings.TrimSpace(v) == "" {
		return MessageID{}, newErr(ErrCodeInvalidMessage, "message ID must not be empty", nil)
	}
	return MessageID{value: v}, nil
}

// MustMessageID panics if v is invalid. Intended for compile-time constants.
func MustMessageID(v string) MessageID {
	id, err := NewMessageID(v)
	if err != nil {
		panic(err)
	}
	return id
}

func (id MessageID) String() string               { return id.value }
func (id MessageID) IsZero() bool                 { return id.value == "" }
func (id MessageID) MarshalJSON() ([]byte, error) { return json.Marshal(id.value) }
func (id *MessageID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := NewMessageID(s)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// ---------------------------------------------------------------------------
// AuthorID
// ---------------------------------------------------------------------------

// AuthorID identifies the entity (agent, user, tool, system) that produced a message.
type AuthorID struct {
	value string
}

// NewAuthorID validates and wraps v in an AuthorID.
func NewAuthorID(v string) (AuthorID, error) {
	if strings.TrimSpace(v) == "" {
		return AuthorID{}, newErr(ErrCodeInvalidMessage, "author ID must not be empty", nil)
	}
	return AuthorID{value: v}, nil
}

func (a AuthorID) String() string               { return a.value }
func (a AuthorID) IsZero() bool                 { return a.value == "" }
func (a AuthorID) MarshalJSON() ([]byte, error) { return json.Marshal(a.value) }
func (a *AuthorID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := NewAuthorID(s)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// ---------------------------------------------------------------------------
// MessageAuthor
// ---------------------------------------------------------------------------

// MessageAuthor is the semantic role of the author in an LLM conversation.
type MessageAuthor string

const (
	AgentAuthor  MessageAuthor = "agent"
	UserAuthor   MessageAuthor = "user"
	SystemAuthor MessageAuthor = "system"
	ToolAuthor   MessageAuthor = "tool"
)

// ---------------------------------------------------------------------------
// MessageType  (canonical alias — defined in llmctx/message)
// ---------------------------------------------------------------------------

// MessageType categorises the payload kind of a Message.
type MessageType = msg.MessageType

const (
	SystemPromptType      MessageType = msg.SystemPromptType
	TextMessageType       MessageType = msg.TextType
	ToolCallMessageType   MessageType = msg.ToolCallType
	ToolResultMessageType MessageType = msg.ToolResultType
	MemoryMessageType     MessageType = msg.MemoryType
	ReasoningMessageType  MessageType = msg.ReasoningType
	LogMessageType        MessageType = msg.LogType
	ErrorMessageType      MessageType = msg.ErrorType
	PlanMessageType       MessageType = msg.PlanType
)

// ---------------------------------------------------------------------------
// MessageWeight
// ---------------------------------------------------------------------------

// MessageWeight expresses eviction priority during compaction.
// Higher value = more protected from eviction.
type MessageWeight struct {
	value int32
}

// Predefined weight sentinels.
var (
	LowestWeight  = MessageWeight{1}
	MediumWeight  = MessageWeight{2}
	HighestWeight = MessageWeight{3}
)

// NewMessageWeight validates and constructs a MessageWeight.
func NewMessageWeight(v int32) (MessageWeight, error) {
	if v < 1 {
		return MessageWeight{}, newErr(ErrCodeInvalidMessage, "weight must be >= 1", nil)
	}
	return MessageWeight{value: v}, nil
}

func (w MessageWeight) Value() int32                      { return w.value }
func (w MessageWeight) IsHigherThan(o MessageWeight) bool { return w.value > o.value }
func (w MessageWeight) Equal(o MessageWeight) bool        { return w.value == o.value }
func (w MessageWeight) MarshalJSON() ([]byte, error)      { return json.Marshal(w.value) }
func (w *MessageWeight) UnmarshalJSON(b []byte) error {
	var v int32
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	parsed, err := NewMessageWeight(v)
	if err != nil {
		return err
	}
	*w = parsed
	return nil
}

// ---------------------------------------------------------------------------
// Timestamp
// ---------------------------------------------------------------------------

// Timestamp gives time.Time a domain-meaningful name. Always stored in UTC.
type Timestamp struct {
	value time.Time
}

// Now returns the current time as a Timestamp.
func Now() Timestamp { return Timestamp{value: time.Now().UTC()} }

// WrapTime wraps an existing time.Time, converting to UTC.
func WrapTime(t time.Time) Timestamp { return Timestamp{value: t.UTC()} }

func (t Timestamp) Time() time.Time               { return t.value }
func (t Timestamp) IsZero() bool                  { return t.value.IsZero() }
func (t Timestamp) Before(o Timestamp) bool       { return t.value.Before(o.value) }
func (t Timestamp) After(o Timestamp) bool        { return t.value.After(o.value) }
func (t Timestamp) Since() time.Duration          { return time.Since(t.value) }
func (t Timestamp) MarshalJSON() ([]byte, error)  { return json.Marshal(t.value) }
func (t *Timestamp) UnmarshalJSON(b []byte) error { return json.Unmarshal(b, &t.value) }
