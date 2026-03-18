package message

import "fmt"

// SystemPromptPayload carries the top-level instructions that define the
// agent's persona, capabilities, and constraints for the session.
//
// Serialised into the provider's dedicated system field (not the messages
// array) by provider-specific adapters.
type SystemPromptPayload struct {
	// Text is the raw system instruction text.
	Text string `json:"text"`
}

func init() { RegisterPayloadFactory(SystemPromptType, func() Payload { return &SystemPromptPayload{} }) }

func (p SystemPromptPayload) Kind() MessageType          { return SystemPromptType }
func (p SystemPromptPayload) TextRepresentation() string { return p.Text }
func (p SystemPromptPayload) sealPayload()               {}

// LLMText returns the system prompt text injected into the model context.
func (p SystemPromptPayload) LLMText() string { return p.Text }

// UserText returns an empty string: system prompts are never shown to users.
func (p SystemPromptPayload) UserText() string { return "" }

// DevText returns a labelled representation for developer tooling.
func (p SystemPromptPayload) DevText() string {
	return fmt.Sprintf("[system_prompt] %s", p.Text)
}
