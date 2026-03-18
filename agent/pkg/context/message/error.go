package message

import "fmt"

// ErrorPayload records a structured error event in the context.
// Agentic loops use this to track failures and enable error-aware reasoning.
type ErrorPayload struct {
	// Code is a machine-readable error code for programmatic handling.
	Code string `json:"code"`
	// Message is the human-readable description.
	Message string `json:"message"`
	// SourceMessageID references the message that triggered the error (optional).
	// Stored as a plain string here to avoid a circular import with the parent
	// llmctx package's MessageID type.
	SourceMessageID string `json:"source_message_id,omitempty"`
	// Retryable indicates whether the failed operation can be retried safely.
	Retryable bool `json:"retryable,omitempty"`
	// Details carries optional structured diagnostic data.
	Details map[string]string `json:"details,omitempty"`
}

func init() { RegisterPayloadFactory(ErrorType, func() Payload { return &ErrorPayload{} }) }

func (p ErrorPayload) Kind() MessageType { return ErrorType }
func (p ErrorPayload) sealPayload()      {}

func (p ErrorPayload) TextRepresentation() string {
	return fmt.Sprintf("[error code=%s] %s", p.Code, p.Message)
}

// LLMText provides a user-readable error summary for model context.
func (p ErrorPayload) LLMText() string {
	return fmt.Sprintf("An error occurred (%s): %s", p.Code, p.Message)
}

// UserText shows the user a friendly error notice.
func (p ErrorPayload) UserText() string {
	if p.Retryable {
		return fmt.Sprintf("⚠️ Something went wrong: %s (you can try again)", p.Message)
	}
	return fmt.Sprintf("❌ Error: %s", p.Message)
}

// DevText returns full diagnostic detail.
func (p ErrorPayload) DevText() string {
	return fmt.Sprintf("[error code=%s retryable=%v source=%s]\n%s\ndetails: %v",
		p.Code, p.Retryable, p.SourceMessageID, p.Message, p.Details)
}
