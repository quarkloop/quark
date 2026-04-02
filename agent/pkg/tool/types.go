// Package tool provides the tool dispatcher and type definitions.
//
// A tool is an executable capability (e.g. web-search, code-interpreter) that
// a subagent agent can invoke. Tools are resolved from the registry at space
// startup and registered with the HTTPDispatcher. The Executor calls
// dispatcher.Invoke during the subagent execution loop.
package tool

// Definition is the resolved tool specification fetched from the registry.
type Definition struct {
	Ref          string                 `json:"ref"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Digest       string                 `json:"digest"`
	Endpoint     string                 `json:"endpoint"`
	InputSchema  map[string]interface{} `json:"input_schema"`
	OutputSchema map[string]interface{} `json:"output_schema"`
	Config       map[string]string      `json:"config"`
}

type InvokeRequest struct {
	ToolName string                 `json:"tool_name"`
	Input    map[string]interface{} `json:"input"`
}

type InvokeResponse struct {
	Output map[string]interface{} `json:"output"`
	Error  string                 `json:"error,omitempty"`
}
