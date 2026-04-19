package plugin

import "context"

// ToolSchema describes a tool's function signature for LLM function calling.
type ToolSchema struct {
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description" yaml:"description"`
	Parameters  map[string]any `json:"parameters" yaml:"parameters"` // JSON Schema
}

// ToolResult is the standardized response from tool execution.
type ToolResult struct {
	Output  map[string]any `json:"output,omitempty"`
	Error   string         `json:"error,omitempty"`
	IsError bool           `json:"is_error"`
}

// ToolPlugin extends Plugin for tools that LLMs can invoke.
// Tools provide executable capabilities like shell execution, file I/O, web search.
type ToolPlugin interface {
	Plugin

	// Schema returns the tool's JSON Schema definition for LLM function calling.
	Schema() ToolSchema

	// Execute runs the tool with the given arguments.
	// Arguments are validated against the schema before execution.
	Execute(ctx context.Context, args map[string]any) (ToolResult, error)
}

// ToolConfig holds tool-specific configuration from the manifest.
type ToolConfig struct {
	Schema      ToolSchema        `yaml:"schema"`
	Permissions PermissionsConfig `yaml:"permissions,omitempty"`
}

// PermissionsConfig declares what resources a tool needs access to.
type PermissionsConfig struct {
	Filesystem []string `yaml:"filesystem,omitempty"` // Allowed filesystem paths
	Network    bool     `yaml:"network"`              // Requires network access
}
