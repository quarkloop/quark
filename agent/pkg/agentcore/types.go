// Package agentcore provides shared types, constants, and resource definitions
// used across all agent sub-packages. It is a leaf package with no business
// logic — only data structures and constants — so every other agent package
// can import it without creating circular dependencies.
package agentcore

// Definition is the agent specification loaded from the Quarkfile and prompt files.
type Definition struct {
	Name         string       `json:"name"`
	SystemPrompt string       `json:"system_prompt"`
	Config       Config       `json:"config"`
	Capabilities Capabilities `json:"capabilities"`
}

// ApprovalPolicy controls whether plans require explicit user approval
// before the supervisor executes them.
type ApprovalPolicy string

const (
	// ApprovalRequired means plans are created as drafts and need explicit
	// user approval before execution. This is the default.
	ApprovalRequired ApprovalPolicy = "required"
	// ApprovalAuto means plans are automatically approved for execution.
	ApprovalAuto ApprovalPolicy = "auto"
)

// Config holds agent configuration values.
type Config struct {
	ContextWindow  int            `json:"context_window"`
	Compaction     string         `json:"compaction"`
	MemoryPolicy   string         `json:"memory_policy"`
	ApprovalPolicy ApprovalPolicy `json:"approval_policy"`
}

// Capabilities declares what an agent is allowed to do.
type Capabilities struct {
	SpawnAgents bool `json:"spawn_agents"`
	MaxWorkers  int  `json:"max_workers"`
	CreatePlans bool `json:"create_plans"`
}

// ChatRequest is the input to Agent.Chat.
type ChatRequest struct {
	Message    string           `json:"message"`
	SessionKey string           `json:"session_key,omitempty"`
	Stream     bool             `json:"stream,omitempty"`
	Mode       string           `json:"mode,omitempty"`
	Files      []FileAttachment `json:"files,omitempty"`
}

// ChatResponse is the output of a non-streaming Agent.Chat call.
type ChatResponse struct {
	Reply        string `json:"reply"`
	Mode         string `json:"mode,omitempty"`
	Warning      string `json:"warning,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

// FileAttachment describes a file uploaded alongside a chat message.
type FileAttachment struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
	Path     string `json:"path,omitempty"`
}
