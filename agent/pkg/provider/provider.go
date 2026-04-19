// Package provider defines the LLM provider interface and wire types.
package provider

import "context"

// Provider is the interface for LLM API providers.
type Provider interface {
	ChatCompletionStream(ctx context.Context, req *Request) (<-chan StreamEvent, error)
	ParseToolCalls(content string) ([]ToolCall, string)
}

// Request is a chat completion request.
type Request struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
	Stream   bool      `json:"stream"`
}

// Message is an LLM chat message (OpenAI-compatible wire format).
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool describes a callable tool for the LLM.
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a function tool.
type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters,omitempty"` // JSON Schema
}

// ToolCall is a tool invocation requested by the LLM.
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

// StreamEvent is a single event from a streaming response.
type StreamEvent struct {
	Delta     string
	ToolCalls []ToolCall
	Done      bool
	Err       error
}
