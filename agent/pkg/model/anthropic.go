package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type anthropicGateway struct {
	model     string
	apiKey    string
	http      *http.Client
	maxTokens int
}

func (g *anthropicGateway) Provider() string       { return "anthropic" }
func (g *anthropicGateway) ModelName() string      { return g.model }
func (g *anthropicGateway) MaxTokens() int         { return g.maxTokens }
func (g *anthropicGateway) Parser() ToolCallParser { return ParserFor("anthropic") }

// anthropicContentBlock represents one block in the Anthropic response content array.
// Blocks can be text (type="text") or tool_use (type="tool_use").
type anthropicContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// anthropicResponse is the minimal response shape we parse.
type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// anthropicModelInfo is the response from GET /v1/models.
type anthropicModelList struct {
	Data []anthropicModelInfo `json:"data"`
}

type anthropicModelInfo struct {
	ID             string `json:"id"`
	MaxTokens      int    `json:"max_tokens"`
	MaxInputTokens int    `json:"max_input_tokens"`
}

// fetchAnthropicMaxTokens queries the Anthropic models API to find the
// max_tokens for the given model name. Returns 0 on any error.
func fetchAnthropicMaxTokens(apiKey, modelName string) int {
	req, err := http.NewRequest(http.MethodGet, "https://api.anthropic.com/v1/models", nil)
	if err != nil {
		return 0
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
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

	var list anthropicModelList
	if err := json.Unmarshal(body, &list); err != nil {
		return 0
	}

	for _, m := range list.Data {
		if m.ID == modelName {
			if m.MaxTokens > 0 {
				return m.MaxTokens
			}
		}
	}
	return 0
}

func newAnthropicGateway(model, apiKey string) (*anthropicGateway, error) {
	maxTokens := fetchAnthropicMaxTokens(apiKey, model)
	if maxTokens <= 0 {
		// Fallback: Anthropic Claude models support 8192 output tokens.
		maxTokens = 8192
	}
	return &anthropicGateway{
		model:     model,
		apiKey:    apiKey,
		http:      newHTTPClient(),
		maxTokens: maxTokens,
	}, nil
}

func (g *anthropicGateway) InferRaw(ctx context.Context, payload []byte) (*RawResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic http: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading anthropic response: %w", err)
	}

	var ar anthropicResponse
	if err := json.Unmarshal(data, &ar); err != nil {
		return nil, fmt.Errorf("decoding anthropic response: %w", err)
	}
	if ar.Error != nil {
		return nil, fmt.Errorf("anthropic api error %s: %s", ar.Error.Type, ar.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic http %d: %s", resp.StatusCode, string(data))
	}

	raw := &RawResponse{
		InputTokens:  ar.Usage.InputTokens,
		OutputTokens: ar.Usage.OutputTokens,
	}
	for _, block := range ar.Content {
		switch block.Type {
		case "text":
			raw.Content += block.Text
		case "tool_use":
			raw.ToolCalls = append(raw.ToolCalls, NativeToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}
	return raw, nil
}
