package model

// ToolDefinition describes a tool for native function calling APIs.
// It mirrors the OpenAI/Anthropic tool schema format.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// BuiltinToolSchemas returns the parameter schemas for quark's built-in tools.
// Used when building the tools array for native function calling requests.
func BuiltinToolSchemas() map[string]ToolDefinition {
	return map[string]ToolDefinition{
		"bash": {
			Name:        "bash",
			Description: "Execute a shell command and return its output.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cmd": map[string]any{
						"type":        "string",
						"description": "The shell command to execute",
					},
				},
				"required": []string{"cmd"},
			},
		},
		"read": {
			Name:        "read",
			Description: "Read the contents of a file.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":       map[string]any{"type": "string", "description": "Absolute file path"},
					"start_line": map[string]any{"type": "integer", "description": "Starting line (1-based, optional)"},
					"end_line":   map[string]any{"type": "integer", "description": "Ending line (1-based, optional)"},
				},
				"required": []string{"path"},
			},
		},
		"write": {
			Name:        "write",
			Description: "Write or edit a file.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":      map[string]any{"type": "string", "description": "Absolute file path"},
					"operation": map[string]any{"type": "string", "enum": []string{"write", "edit"}, "description": "write=create/overwrite, edit=partial edits"},
					"content":   map[string]any{"type": "string", "description": "File content for write operation"},
					"edits":     map[string]any{"type": "array", "description": "Edit operations for edit mode"},
				},
				"required": []string{"path", "operation"},
			},
		},
		"web_search": {
			Name:        "web_search",
			Description: "Search the web and return results.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search query"},
				},
				"required": []string{"query"},
			},
		},
	}
}
