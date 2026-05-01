// Package llm provides the high-level inference loop.
package llm

import (
	"context"

	"github.com/quarkloop/pkg/plugin"
)

// Provider is the minimal interface for LLM API providers.
// It is a reduced subset of plugin.ProviderPlugin that excludes lifecycle
// methods (Configure, Shutdown) and metadata (ProviderID, ListModels).
type Provider interface {
	// ChatCompletionStream sends a streaming chat completion request.
	ChatCompletionStream(ctx context.Context, req *plugin.ChatRequest) (<-chan plugin.StreamEvent, error)
	// ParseToolCalls extracts tool calls from content (for non-native tool calling).
	ParseToolCalls(content string) ([]plugin.ToolCall, string)
}

// ProviderAdapter wraps a plugin.ProviderPlugin to satisfy the minimal Provider interface.
type ProviderAdapter struct {
	plug plugin.ProviderPlugin
}

// NewProviderAdapter creates an adapter for a provider plugin.
func NewProviderAdapter(p plugin.ProviderPlugin) *ProviderAdapter {
	return &ProviderAdapter{plug: p}
}

// ChatCompletionStream delegates to the plugin.
func (a *ProviderAdapter) ChatCompletionStream(ctx context.Context, req *plugin.ChatRequest) (<-chan plugin.StreamEvent, error) {
	return a.plug.ChatCompletionStream(ctx, req)
}

// ParseToolCalls delegates to the plugin.
func (a *ProviderAdapter) ParseToolCalls(content string) ([]plugin.ToolCall, string) {
	return a.plug.ParseToolCalls(content)
}
