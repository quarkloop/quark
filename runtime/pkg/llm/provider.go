// Package llm provides the high-level inference loop.
package llm

import (
	"context"

	"github.com/quarkloop/pkg/plugin"
)

// Provider is an alias for plugin.Provider, the minimal interface for LLM API providers.
type Provider = plugin.Provider

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
