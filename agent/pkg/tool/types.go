// Package tool provides the tool registry and type definitions.
//
// A tool is an executable capability (e.g. bash, read, write, web-search) that
// a subagent can invoke. Tools are resolved from the Quarkfile at space startup
// and registered with the Registry. The Executor calls Registry.Invoke during
// the subagent execution loop.
package tool

import (
	"context"
	"time"
)

// Invoker is the interface that dispatches tool calls.
type Invoker interface {
	Register(name string, def *Definition)
	Invoke(ctx context.Context, name string, input map[string]any) (map[string]any, error)
	List() []string
}

// Definition is the resolved tool specification fetched from the registry.
type Definition struct {
	Ref          string            `json:"ref"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Digest       string            `json:"digest"`
	Endpoint     string            `json:"endpoint"`
	InputSchema  map[string]any    `json:"input_schema"`
	OutputSchema map[string]any    `json:"output_schema"`
	Config       map[string]string `json:"config"`
	Description  string            `json:"description"`
	Group        string            `json:"group"`
	Hidden       bool              `json:"hidden"`
	TTL          time.Duration     `json:"ttl"`
	RegisteredAt time.Time         `json:"registered_at"`
	ExpiresAt    *time.Time        `json:"expires_at"`
}

// ToolStats tracks usage metrics for a registered tool.
type ToolStats struct {
	CallCount    int64         `json:"call_count"`
	ErrorCount   int64         `json:"error_count"`
	LastCalled   time.Time     `json:"last_called"`
	AvgLatencyMs float64       `json:"avg_latency_ms"`
	TotalLatency time.Duration `json:"-"`
}

type InvokeRequest struct {
	ToolName string         `json:"tool_name"`
	Input    map[string]any `json:"input"`
}

type InvokeResponse struct {
	Output map[string]any `json:"output"`
	Error  string         `json:"error,omitempty"`
}
