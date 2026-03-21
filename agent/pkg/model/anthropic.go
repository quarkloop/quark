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
	model  string
	apiKey string
	http   *http.Client
}

func (g *anthropicGateway) Provider() string         { return "anthropic" }
func (g *anthropicGateway) ModelName() string        { return g.model }
func (g *anthropicGateway) MaxTokens() int           { return 8192 }
func (g *anthropicGateway) Parser() ToolCallParser   { return ParserFor("anthropic") }

// anthropicResponse is the minimal response shape we parse.
type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
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

	content := ""
	for _, block := range ar.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}
	return &RawResponse{
		Content:      content,
		InputTokens:  ar.Usage.InputTokens,
		OutputTokens: ar.Usage.OutputTokens,
	}, nil
}
