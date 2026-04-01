package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type zhipuGateway struct {
	model     string
	apiKey    string
	http      *http.Client
	maxTokens int
}

func (g *zhipuGateway) Provider() string       { return "zhipu" }
func (g *zhipuGateway) ModelName() string      { return g.model }
func (g *zhipuGateway) MaxTokens() int         { return g.maxTokens }
func (g *zhipuGateway) Parser() ToolCallParser { return ParserFor("zhipu") }

type zhipuResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func (g *zhipuGateway) InferRaw(ctx context.Context, payload []byte) (*RawResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://open.bigmodel.cn/api/paas/v4/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("zhipu http: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading zhipu response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zhipu http %d: %s", resp.StatusCode, string(data))
	}

	var zr zhipuResponse
	if err := json.Unmarshal(data, &zr); err != nil {
		return nil, fmt.Errorf("decoding zhipu response: %w", err)
	}
	if zr.Error != nil {
		return nil, fmt.Errorf("zhipu api error %s: %s", zr.Error.Code, zr.Error.Message)
	}
	if len(zr.Choices) == 0 {
		return nil, fmt.Errorf("zhipu returned no choices")
	}
	return &RawResponse{
		Content:      zr.Choices[0].Message.Content,
		InputTokens:  zr.Usage.PromptTokens,
		OutputTokens: zr.Usage.CompletionTokens,
	}, nil
}
