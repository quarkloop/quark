// Package message defines the sealed Payload hierarchy used by the llmctx
// context management package.
//
// Every concrete payload type lives in its own file in this package.  The
// Payload interface is sealed: only types within this package can implement it
// (the unexported sealPayload method acts as the compile-time seal).
//
// Adding a new payload type:
//  1. Create a new file <name>.go in this package.
//  2. Define the struct with exported, JSON-tagged fields.
//  3. Implement Kind, TextRepresentation, LLMText, UserText, DevText, sealPayload.
//  4. Add a MessageType constant to the block in this file.
//  5. Register the type in init() via RegisterPayloadFactory.
//  6. Add a New*Message factory in the parent llmctx package (message.go).
//  7. Add a default visibility entry in llmctx/visibility.go.
package message

import (
	"encoding/json"
	"fmt"
)

// =============================================================================
// MessageType
// =============================================================================

// MessageType identifies the kind of payload a Message carries.
// Constants are defined here so they are co-located with the types they name.
type MessageType string

const (
	SystemPromptType      MessageType = "system_prompt"
	TextType              MessageType = "text"
	ImageType             MessageType = "image"
	PDFType               MessageType = "pdf"
	AudioType             MessageType = "audio"
	ToolCallType          MessageType = "tool_call"
	ToolResultType        MessageType = "tool_result"
	MemoryType            MessageType = "memory"
	ReasoningType         MessageType = "reasoning"
	LogType               MessageType = "log"
	ErrorType             MessageType = "error"
	PlanType              MessageType = "plan"
)

// =============================================================================
// Payload — sealed interface
// =============================================================================

// Payload is the typed content envelope carried by a Message.
//
// Sealed: only types in this package implement it via the unexported
// sealPayload method.  This guarantees exhaustive handling in type switches
// and prevents external misuse.
//
// The three render methods (LLMText, UserText, DevText) implement the
// "visibility as rendering target" pattern: each surface receives a
// representation tailored to its audience rather than a single flat string.
type Payload interface {
	// Kind returns the MessageType constant that identifies this payload.
	Kind() MessageType

	// TextRepresentation returns a stable flat-string form used for:
	//   - token counting (all tokenisers call this)
	//   - ToFlatString serialisation
	//   - debug logging
	// Must never be empty for non-empty payloads.
	TextRepresentation() string

	// LLMText returns the string injected into the LLM conversation turn.
	// For types that must never reach the model (LogPayload, etc.) return "".
	LLMText() string

	// UserText returns the string surfaced in the end-user chat interface.
	// Return "" to hide from users.
	UserText() string

	// DevText returns the string shown in developer/debug tooling.
	// Should include full structural detail.
	DevText() string

	// sealPayload prevents external packages from implementing Payload.
	sealPayload()
}

// =============================================================================
// Payload factory registry
// =============================================================================

// PayloadFactory constructs a zero-value Payload of a specific type, ready
// for json.Unmarshal to populate.
type PayloadFactory func() Payload

var registry = map[MessageType]PayloadFactory{}

// RegisterPayloadFactory registers a factory for a MessageType.
// Called from init() in each payload file.
// Panics if the same type is registered twice (programming error).
func RegisterPayloadFactory(t MessageType, f PayloadFactory) {
	if _, exists := registry[t]; exists {
		panic(fmt.Sprintf("message: duplicate payload factory registration for %q", t))
	}
	registry[t] = f
}

// UnmarshalPayload decodes raw JSON bytes into the concrete Payload type
// identified by kind.  Returns an error for unrecognised kinds.
//
// The returned value is always a value type (not *T) to match what the
// New*Message factories store, ensuring As*() type assertions succeed.
func UnmarshalPayload(kind MessageType, raw json.RawMessage) (Payload, error) {
	f, ok := registry[kind]
	if !ok {
		return nil, fmt.Errorf("message: unknown payload kind %q", kind)
	}
	dst := f()
	if err := json.Unmarshal(raw, dst); err != nil {
		return nil, fmt.Errorf("message: failed to decode %q payload: %w", kind, err)
	}
	// dst is a *T; dereference to T so the interface holds the value type.
	return derefPayload(dst), nil
}

// derefPayload strips one pointer indirection from a Payload value.
// All concrete payload types use value receivers; factories return *T so that
// json.Unmarshal can populate fields.  We dereference before storing in the
// interface so that type assertions like payload.(TextPayload) succeed.
func derefPayload(p Payload) Payload {
	switch v := p.(type) {
	case *SystemPromptPayload:
		return *v
	case *TextPayload:
		return *v
	case *ImagePayload:
		return *v
	case *PDFPayload:
		return *v
	case *AudioPayload:
		return *v
	case *ToolCallPayload:
		return *v
	case *ToolResultPayload:
		return *v
	case *MemoryPayload:
		return *v
	case *ReasoningPayload:
		return *v
	case *LogPayload:
		return *v
	case *ErrorPayload:
		return *v
	case *PlanPayload:
		return *v
	default:
		// Unknown type — return as-is; the caller will handle any mismatch.
		return p
	}
}
