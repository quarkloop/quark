// Package llm provides the high-level inference loop.
//
// The Infer function implements the full call order:
//  1. Call LLM with context (streaming)
//  2. If LLM returns tool calls → execute tools → feed results back → loop
//  3. Stream text tokens to response channel
//  4. Return full assistant response
package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/quarkloop/pkg/plugin"
)

// Client wraps a provider with the inference loop.
type Client struct {
	provider      Provider
	model         string
	ContextWindow int // token limit from the model entry (0 = unknown)
}

// NewClient creates a new LLM client.
func NewClient(p Provider, model string, contextWindow int) *Client {
	return &Client{provider: p, model: model, ContextWindow: contextWindow}
}

// Infer runs the full inference loop:
//
//	context → LLM call → tool handling → response streaming.
//
// It fires onMessage for streamed text and tool execution data.
// If onTool is nil, tool calls are ignored.
func (c *Client) Infer(ctx context.Context, messages []plugin.Message, tools []plugin.ToolSchema, onTool plugin.ToolHandler, onMessage func(msgType string, data any)) (string, error) {
	for {
		stream, err := c.provider.ChatCompletionStream(ctx, &plugin.ChatRequest{
			Model:    c.model,
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			return "", fmt.Errorf("llm call: %w", err)
		}

		var fullContent string
		var toolCalls []plugin.ToolCall

		for ev := range stream {
			if ev.Err != nil {
				return "", fmt.Errorf("stream: %w", ev.Err)
			}
			if ev.Done {
				break
			}
			if ev.Delta != "" {
				fullContent += ev.Delta
				if onMessage != nil {
					onMessage("token", ev.Delta)
				}
			}
			toolCalls = mergeToolCalls(toolCalls, ev.ToolCalls)
		}

		// No native tool calls — try parsing from text output
		if len(toolCalls) == 0 && c.provider != nil {
			parsedTools, cleaned := c.provider.ParseToolCalls(fullContent)
			if len(parsedTools) > 0 {
				toolCalls = parsedTools
				fullContent = strings.TrimSpace(cleaned)
			}
		}

		// No tool calls at all — we're done
		if len(toolCalls) == 0 {
			return fullContent, nil
		}

		// No handler — return what we have
		if onTool == nil {
			return fullContent, nil
		}

		slog.Info("tool calls", "count", len(toolCalls), "names", toolCallNames(toolCalls))

		// Append assistant message with tool calls
		messages = append(messages, plugin.Message{
			Role:      "assistant",
			Content:   fullContent,
			ToolCalls: toolCalls,
		})

		// Execute each tool and append results
		for _, tc := range toolCalls {
			if onMessage != nil {
				onMessage("tool_start", map[string]string{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				})
			}

			result, err := onTool(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("error: %v", err)
			}
			if onMessage != nil {
				onMessage("tool_result", map[string]string{
					"name":   tc.Function.Name,
					"result": preview(result, 2000),
				})
			}
			messages = append(messages, plugin.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}

		// Reset for next LLM call
		fullContent = ""
	}
}

// mergeToolCalls accumulates streamed tool call deltas by index.
func mergeToolCalls(existing []plugin.ToolCall, deltas []plugin.ToolCall) []plugin.ToolCall {
	for _, d := range deltas {
		idx := d.Index

		// Grow slice to fit
		for len(existing) <= idx {
			existing = append(existing, plugin.ToolCall{})
		}

		tc := &existing[idx]
		tc.Index = idx // CRITICAL: Retain the proper loop index!
		if d.ID != "" {
			tc.ID = d.ID
		}
		if d.Type != "" {
			tc.Type = d.Type
		}
		if d.Function.Name != "" {
			tc.Function.Name = d.Function.Name
		}
		tc.Function.Arguments += d.Function.Arguments
	}
	return existing
}

func toolCallNames(calls []plugin.ToolCall) []string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Function.Name)
	}
	return names
}

func preview(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "...(truncated)"
}
