package plugin

import "context"

// Message is an LLM chat message (OpenAI-compatible wire format).
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents an LLM's request to invoke a tool.
type ToolCall struct {
	Index    int              `json:"index"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"` // "function"
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the function name and arguments.
type ToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ChatRequest is a chat completion request.
type ChatRequest struct {
	Model    string       `json:"model"`
	Messages []Message    `json:"messages"`
	Tools    []ToolSchema `json:"tools,omitempty"`
	Stream   bool         `json:"stream"`
}

// StreamEvent is a single event from a streaming response.
type StreamEvent struct {
	Delta     string     // Text delta
	ToolCalls []ToolCall // Tool call deltas
	Done      bool       // Stream complete
	Err       error      // Error if any
}

// ModelInfo describes an available model from a provider.
type ModelInfo struct {
	ID            string `json:"id" yaml:"id"`
	Name          string `json:"name" yaml:"name"`
	ContextWindow int    `json:"context_window" yaml:"context_window"`
	Default       bool   `json:"default,omitempty" yaml:"default,omitempty"`
}

// ProviderConfig holds configuration for a provider plugin.
type ProviderConfig struct {
	APIKey    string            `json:"api_key"`
	BaseURL   string            `json:"base_url,omitempty"`
	ExtraOpts map[string]string `json:"extra_opts,omitempty"`
}

// ProviderPlugin extends Plugin for LLM API providers.
// Providers handle communication with LLM services like OpenAI, Anthropic, OpenRouter.
type ProviderPlugin interface {
	Plugin

	// ProviderID returns the unique provider identifier (e.g., "openrouter", "openai").
	ProviderID() string

	// Configure sets provider-specific configuration (API keys, base URLs).
	Configure(config ProviderConfig) error

	// ListModels returns available models from this provider.
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// ChatCompletionStream sends a streaming chat completion request.
	ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)

	// ParseToolCalls extracts tool calls from content (for non-native tool calling).
	// Returns the extracted tool calls and the remaining content.
	ParseToolCalls(content string) ([]ToolCall, string)
}

// ProviderManifestConfig holds provider-specific configuration from the manifest.
type ProviderManifestConfig struct {
	APIBase             string      `yaml:"api_base"`
	AuthEnv             string      `yaml:"auth_env"` // Environment variable for API key
	ModelsEndpoint      string      `yaml:"models_endpoint,omitempty"`
	SupportsNativeTools bool        `yaml:"supports_native_tools"`
	SupportsStreaming   bool        `yaml:"supports_streaming"`
	DefaultModel        string      `yaml:"default_model,omitempty"`
	Models              []ModelInfo `yaml:"models,omitempty"` // Static model list
}
