package pluginmanager

import (
	"context"

	"github.com/quarkloop/agent/pkg/provider"
	"github.com/quarkloop/pkg/plugin"
)

// ProviderAdapter wraps a plugin.ProviderPlugin to implement provider.Provider.
// This allows provider plugins to be used with the existing LLM registry.
type ProviderAdapter struct {
	plug plugin.ProviderPlugin
}

// NewProviderAdapter creates an adapter for a provider plugin.
func NewProviderAdapter(p plugin.ProviderPlugin) *ProviderAdapter {
	return &ProviderAdapter{plug: p}
}

// ChatCompletionStream converts types and delegates to the plugin.
func (a *ProviderAdapter) ChatCompletionStream(ctx context.Context, req *provider.Request) (<-chan provider.StreamEvent, error) {
	// Convert provider.Request to plugin.ChatRequest
	pluginReq := &plugin.ChatRequest{
		Model:    req.Model,
		Messages: convertMessagesToPlugin(req.Messages),
		Tools:    convertToolsToPlugin(req.Tools),
		Stream:   req.Stream,
	}

	// Call the plugin
	pluginCh, err := a.plug.ChatCompletionStream(ctx, pluginReq)
	if err != nil {
		return nil, err
	}

	// Convert plugin.StreamEvent to provider.StreamEvent
	ch := make(chan provider.StreamEvent, 64)
	go func() {
		defer close(ch)
		for event := range pluginCh {
			ch <- provider.StreamEvent{
				Delta:     event.Delta,
				ToolCalls: convertToolCallsFromPlugin(event.ToolCalls),
				Done:      event.Done,
				Err:       event.Err,
			}
		}
	}()

	return ch, nil
}

// ParseToolCalls delegates to the plugin.
func (a *ProviderAdapter) ParseToolCalls(content string) ([]provider.ToolCall, string) {
	calls, remaining := a.plug.ParseToolCalls(content)
	return convertToolCallsFromPlugin(calls), remaining
}

// --- Type conversion helpers ---

func convertMessagesToPlugin(msgs []provider.Message) []plugin.Message {
	out := make([]plugin.Message, len(msgs))
	for i, m := range msgs {
		out[i] = plugin.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCalls:  convertToolCallsToPlugin(m.ToolCalls),
			ToolCallID: m.ToolCallID,
		}
	}
	return out
}

func convertToolsToPlugin(tools []provider.Tool) []plugin.ToolSchema {
	out := make([]plugin.ToolSchema, len(tools))
	for i, t := range tools {
		params, _ := t.Function.Parameters.(map[string]any)
		out[i] = plugin.ToolSchema{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  params,
		}
	}
	return out
}

func convertToolCallsToPlugin(tcs []provider.ToolCall) []plugin.ToolCall {
	out := make([]plugin.ToolCall, len(tcs))
	for i, tc := range tcs {
		out[i] = plugin.ToolCall{
			Index: tc.Index,
			ID:    tc.ID,
			Type:  tc.Type,
			Function: plugin.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return out
}

func convertToolCallsFromPlugin(tcs []plugin.ToolCall) []provider.ToolCall {
	out := make([]provider.ToolCall, len(tcs))
	for i, tc := range tcs {
		out[i] = provider.ToolCall{
			Index: tc.Index,
			ID:    tc.ID,
			Type:  tc.Type,
			Function: provider.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return out
}
