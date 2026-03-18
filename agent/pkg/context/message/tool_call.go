package message

import (
	"encoding/json"
	"fmt"
)

// ToolCallPayload records the LLM's decision to invoke a tool/function.
//
// After appending this to the context, the agent must execute the tool and
// append a corresponding ToolResultPayload with the same ToolCallID.
type ToolCallPayload struct {
	// ToolCallID is the provider-assigned correlation key.
	// Must match the ToolCallID on the corresponding ToolResultPayload.
	ToolCallID string `json:"tool_call_id"`
	// ToolName is the function/tool being invoked.
	ToolName string `json:"tool_name"`
	// Arguments is the raw JSON object of arguments produced by the LLM.
	Arguments json.RawMessage `json:"arguments"`
}

func init() { RegisterPayloadFactory(ToolCallType, func() Payload { return &ToolCallPayload{} }) }

func (p ToolCallPayload) Kind() MessageType { return ToolCallType }
func (p ToolCallPayload) sealPayload()      {}

func (p ToolCallPayload) TextRepresentation() string {
	return fmt.Sprintf("[tool_call id=%s name=%s args=%s]",
		p.ToolCallID, p.ToolName, string(p.Arguments))
}

// LLMText renders the call as a function-call string for the model context.
func (p ToolCallPayload) LLMText() string {
	return fmt.Sprintf("Tool call: %s(%s)", p.ToolName, string(p.Arguments))
}

// UserText shows a brief "Checking…" label — users don't need the raw args.
func (p ToolCallPayload) UserText() string {
	return fmt.Sprintf("🔧 Calling %s…", p.ToolName)
}

// DevText returns full structural detail including the argument payload.
func (p ToolCallPayload) DevText() string {
	return fmt.Sprintf("[tool_call id=%s name=%s]\nargs: %s",
		p.ToolCallID, p.ToolName, string(p.Arguments))
}
