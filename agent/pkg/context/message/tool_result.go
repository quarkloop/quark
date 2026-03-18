package message

import "fmt"

// ToolResultPayload carries the output of a tool execution back to the LLM.
// Must reference the ToolCallID from the corresponding ToolCallPayload.
type ToolResultPayload struct {
	// ToolCallID links this result to its originating ToolCallPayload.
	ToolCallID string `json:"tool_call_id"`
	// ToolName is the tool that produced this result (redundant but useful for logging).
	ToolName string `json:"tool_name"`
	// Content is the tool's textual output.
	Content string `json:"content"`
	// IsError signals that the tool execution failed.
	IsError bool `json:"is_error,omitempty"`
	// ErrorMessage contains the failure description when IsError is true.
	ErrorMessage string `json:"error_message,omitempty"`
}

func init() {
	RegisterPayloadFactory(ToolResultType, func() Payload { return &ToolResultPayload{} })
}

func (p ToolResultPayload) Kind() MessageType { return ToolResultType }
func (p ToolResultPayload) sealPayload()      {}

func (p ToolResultPayload) TextRepresentation() string {
	if p.IsError {
		return fmt.Sprintf("[tool_result id=%s name=%s ERROR: %s]",
			p.ToolCallID, p.ToolName, p.ErrorMessage)
	}
	return fmt.Sprintf("[tool_result id=%s name=%s] %s",
		p.ToolCallID, p.ToolName, p.Content)
}

// LLMText returns the tool output that the model needs to continue reasoning.
func (p ToolResultPayload) LLMText() string {
	if p.IsError {
		return fmt.Sprintf("Tool %q error: %s", p.ToolName, p.ErrorMessage)
	}
	return p.Content
}

// UserText returns a brief success/error label.
func (p ToolResultPayload) UserText() string {
	if p.IsError {
		return fmt.Sprintf("❌ %s failed: %s", p.ToolName, p.ErrorMessage)
	}
	return fmt.Sprintf("✅ %s result received", p.ToolName)
}

// DevText returns full structural detail.
func (p ToolResultPayload) DevText() string {
	if p.IsError {
		return fmt.Sprintf("[tool_result id=%s name=%s is_error=true]\nerror: %s",
			p.ToolCallID, p.ToolName, p.ErrorMessage)
	}
	return fmt.Sprintf("[tool_result id=%s name=%s]\ncontent: %s",
		p.ToolCallID, p.ToolName, p.Content)
}
