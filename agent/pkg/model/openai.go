package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openAIBaseURL = "https://api.openai.com/v1/chat/completions"

type openAIGateway struct {
	provider string // "openai", "openrouter", etc.
	baseURL  string
	model    string
	apiKey   string
	http     *http.Client
}

func (g *openAIGateway) Provider() string         { return g.provider }
func (g *openAIGateway) ModelName() string        { return g.model }
func (g *openAIGateway) MaxTokens() int           { return 4096 }
func (g *openAIGateway) Parser() ToolCallParser   { return ParserFor(g.provider) }

type openAIToolCallEntry struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content   string               `json:"content"`
			ToolCalls []openAIToolCallEntry `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (g *openAIGateway) InferRaw(ctx context.Context, payload []byte) (*RawResponse, error) {
	url := g.baseURL
	if url == "" {
		url = openAIBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai http: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading openai response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s http %d: %s", g.provider, resp.StatusCode, string(data))
	}

	var or openAIResponse
	if err := json.Unmarshal(data, &or); err != nil {
		return nil, fmt.Errorf("decoding %s response: %w", g.provider, err)
	}
	if or.Error != nil {
		return nil, fmt.Errorf("%s api error %s: %s", g.provider, or.Error.Type, or.Error.Message)
	}
	if len(or.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}
	raw := &RawResponse{
		Content:      or.Choices[0].Message.Content,
		InputTokens:  or.Usage.PromptTokens,
		OutputTokens: or.Usage.CompletionTokens,
	}
	for _, tc := range or.Choices[0].Message.ToolCalls {
		raw.ToolCalls = append(raw.ToolCalls, NativeToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}
	return raw, nil
}
