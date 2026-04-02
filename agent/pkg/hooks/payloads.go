package hooks

// ToolCallPayload is passed to BeforeToolCall hooks.
type ToolCallPayload struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
	StepID    string                 `json:"step_id"`
	SessionID string                 `json:"session_id"`
}

// ToolResultPayload is passed to AfterToolCall hooks.
type ToolResultPayload struct {
	ToolName string `json:"tool_name"`
	Content  string `json:"content"`
	IsError  bool   `json:"is_error"`
	StepID   string `json:"step_id"`
}

// InferencePayload is passed to BeforeInference hooks.
type InferencePayload struct {
	Model     string `json:"model"`
	Provider  string `json:"provider"`
	UserMsg   string `json:"user_message"`
	SessionID string `json:"session_id"`
}

// InferenceResultPayload is passed to AfterInference hooks.
type InferenceResultPayload struct {
	Content      string `json:"content"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}
