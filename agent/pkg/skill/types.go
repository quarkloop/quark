// Package skill provides the skill dispatcher and type definitions.
//
// A skill is an executable capability (e.g. web-search, code-interpreter) that
// a worker agent can invoke. Skills are resolved from the registry at space
// startup and registered with the HTTPDispatcher. The Executor calls
// dispatcher.Invoke during the worker execution loop.
package skill

// Definition is the resolved skill specification fetched from the registry.
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
	SkillName string                 `json:"skill_name"`
	Input     map[string]interface{} `json:"input"`
}

type InvokeResponse struct {
	Output map[string]interface{} `json:"output"`
	Error  string                 `json:"error,omitempty"`
}
