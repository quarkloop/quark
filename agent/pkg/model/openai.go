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

// openaiModelDefaults maps well-known model names to their max output tokens.
// Used as fallback when the API doesn't expose this info (OpenAI) or the
// model isn't found in the OpenRouter catalog.
var openaiModelDefaults = map[string]int{
	// GPT-4o family
	"gpt-4o":            16384,
	"gpt-4o-mini":       16384,
	"gpt-4o-2024-":      16384,
	"gpt-4o-mini-2024-": 16384,
	// GPT-4 Turbo
	"gpt-4-turbo":       4096,
	"gpt-4-turbo-2024-": 4096,
	// GPT-4
	"gpt-4":     8192,
	"gpt-4-32k": 8192,
	// o-series (reasoning)
	"o1":         100000,
	"o1-mini":    65536,
	"o1-preview": 32768,
	"o3":         100000,
	"o3-mini":    100000,
	"o4-mini":    100000,
}

type openAIGateway struct {
	provider  string // "openai", "openrouter", etc.
	baseURL   string
	model     string
	apiKey    string
	http      *http.Client
	maxTokens int
}

func (g *openAIGateway) Provider() string       { return g.provider }
func (g *openAIGateway) ModelName() string      { return g.model }
func (g *openAIGateway) MaxTokens() int         { return g.maxTokens }
func (g *openAIGateway) Parser() ToolCallParser { return ParserFor(g.provider) }

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
			Content   string                `json:"content"`
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

// fetchOpenRouterMaxTokens queries the OpenRouter models API to find the
// max_completion_tokens for the given model. Returns 0 on any error.
func fetchOpenRouterMaxTokens(modelName string) int {
	resp, err := http.Get("https://openrouter.ai/api/v1/models")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0
	}

	var list struct {
		Data []struct {
			ID          string `json:"id"`
			TopProvider struct {
				MaxCompletionTokens int `json:"max_completion_tokens"`
			} `json:"top_provider"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return 0
	}

	for _, m := range list.Data {
		if m.ID == modelName {
			if m.TopProvider.MaxCompletionTokens > 0 {
				return m.TopProvider.MaxCompletionTokens
			}
		}
	}
	return 0
}

// resolveMaxTokens determines the max output tokens for the given provider/model.
// - OpenRouter: fetches from the public models API, falls back to defaults table.
// - OpenAI: uses the defaults table (OpenAI's model info endpoint doesn't expose max_tokens).
func resolveMaxTokens(provider, modelName string) int {
	switch provider {
	case "openrouter":
		if n := fetchOpenRouterMaxTokens(modelName); n > 0 {
			return n
		}
		// Fallback: scan defaults table for prefix match.
		for prefix, n := range openaiModelDefaults {
			if len(modelName) >= len(prefix) && modelName[:len(prefix)] == prefix {
				return n
			}
		}
		return 8192 // safe default
	case "openai":
		// Direct match first.
		if n, ok := openaiModelDefaults[modelName]; ok {
			return n
		}
		// Prefix match for versioned model names.
		for prefix, n := range openaiModelDefaults {
			if len(modelName) >= len(prefix) && modelName[:len(prefix)] == prefix {
				return n
			}
		}
		return 8192 // safe default
	default:
		return 8192
	}
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
