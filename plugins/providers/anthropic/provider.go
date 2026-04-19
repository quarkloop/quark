//go:build plugin

// Package main implements the Anthropic provider plugin.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/quarkloop/pkg/plugin"
)

const (
	defaultBaseURL      = "https://api.anthropic.com/v1"
	anthropicAPIVersion = "2023-06-01"
)

// AnthropicProvider implements the ProviderPlugin interface for Anthropic.
type AnthropicProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) Version() string {
	return "1.0.0"
}

func (p *AnthropicProvider) Type() plugin.PluginType {
	return plugin.TypeProvider
}

func (p *AnthropicProvider) ProviderID() string {
	return "anthropic"
}

func (p *AnthropicProvider) Initialize(ctx context.Context, config map[string]any) error {
	p.client = &http.Client{}
	p.baseURL = defaultBaseURL
	return nil
}

func (p *AnthropicProvider) Shutdown(ctx context.Context) error {
	return nil
}

func (p *AnthropicProvider) Configure(config plugin.ProviderConfig) error {
	p.apiKey = config.APIKey
	if config.BaseURL != "" {
		p.baseURL = config.BaseURL
	}
	return nil
}

func (p *AnthropicProvider) ListModels(ctx context.Context) ([]plugin.ModelInfo, error) {
	return []plugin.ModelInfo{
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", ContextWindow: 200000, Default: true},
		{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", ContextWindow: 200000},
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", ContextWindow: 200000},
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", ContextWindow: 200000},
		{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", ContextWindow: 200000},
	}, nil
}

// ChatCompletionStream sends a streaming chat completion request to Anthropic.
func (p *AnthropicProvider) ChatCompletionStream(ctx context.Context, req *plugin.ChatRequest) (<-chan plugin.StreamEvent, error) {
	// Convert to Anthropic Messages API format
	antReq := &anthropicRequest{
		Model:     req.Model,
		MaxTokens: 8192, // Required field for Anthropic
		Stream:    true,
	}

	// Extract system message and convert other messages
	for _, m := range req.Messages {
		if m.Role == "system" {
			antReq.System = m.Content
		} else {
			antReq.Messages = append(antReq.Messages, anthropicMsg{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	if len(req.Tools) > 0 {
		antReq.Tools = convertTools(req.Tools)
	}

	body, err := json.Marshal(antReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("anthropic: %d %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan plugin.StreamEvent, 64)
	go p.readStream(resp.Body, ch)
	return ch, nil
}

// readStream parses Anthropic's SSE stream into StreamEvents.
func (p *AnthropicProvider) readStream(body io.ReadCloser, ch chan<- plugin.StreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var currentToolCallIndex int
	var toolCalls []plugin.ToolCall

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var event anthropicEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				ch <- plugin.StreamEvent{Delta: event.Delta.Text}
			} else if event.Delta.Type == "input_json_delta" {
				// Accumulate tool call arguments
				if len(toolCalls) > 0 {
					toolCalls[len(toolCalls)-1].Function.Arguments += event.Delta.PartialJSON
				}
			}

		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				toolCalls = append(toolCalls, plugin.ToolCall{
					Index: currentToolCallIndex,
					ID:    event.ContentBlock.ID,
					Type:  "function",
					Function: plugin.ToolCallFunction{
						Name:      event.ContentBlock.Name,
						Arguments: "",
					},
				})
				currentToolCallIndex++
			}

		case "content_block_stop":
			// Send accumulated tool calls if any
			if len(toolCalls) > 0 {
				ch <- plugin.StreamEvent{ToolCalls: toolCalls}
				toolCalls = nil
			}

		case "message_stop":
			ch <- plugin.StreamEvent{Done: true}
			return

		case "error":
			ch <- plugin.StreamEvent{Err: fmt.Errorf("anthropic: %s", event.Error.Message)}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- plugin.StreamEvent{Err: fmt.Errorf("read stream: %w", err)}
	}
}

// ParseToolCalls - Anthropic uses native tool calling, so this is a no-op.
func (p *AnthropicProvider) ParseToolCalls(content string) ([]plugin.ToolCall, string) {
	return nil, content
}

// --- Request/Response types for Anthropic Messages API ---

type anthropicRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []anthropicMsg  `json:"messages"`
	Tools     []anthropicTool `json:"tools,omitempty"`
	Stream    bool            `json:"stream"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicEvent struct {
	Type         string                `json:"type"`
	Delta        anthropicDelta        `json:"delta,omitempty"`
	ContentBlock anthropicContentBlock `json:"content_block,omitempty"`
	Error        anthropicError        `json:"error,omitempty"`
}

type anthropicDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// --- Conversion helpers ---

func convertTools(schemas []plugin.ToolSchema) []anthropicTool {
	out := make([]anthropicTool, len(schemas))
	for i, s := range schemas {
		out[i] = anthropicTool{
			Name:        s.Name,
			Description: s.Description,
			InputSchema: s.Parameters,
		}
	}
	return out
}
