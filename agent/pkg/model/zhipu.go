package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// zhipuModelDefaults maps GLM model name prefixes to their max output tokens.
// Zhipu's API doesn't expose this programmatically, so we maintain this table.
var zhipuModelDefaults = map[string]int{
	"GLM-4-Plus":   4096,
	"GLM-4":        4096,
	"GLM-4-Flash":  4096,
	"GLM-4-FlashX": 4096,
	"GLM-4-Air":    4096,
	"GLM-4-AirX":   4096,
	"GLM-4-Long":   4096,
	"GLM-4V":       4096,
	"GLM-4V-Flash": 4096,
	"codegeex-4":   4096,
	"embedding-3":  4096,
}

// resolveZhipuMaxTokens returns the max output tokens for a GLM model name.
// Falls back to 8192 for unknown models.
func resolveZhipuMaxTokens(modelName string) int {
	// Direct match.
	if n, ok := zhipuModelDefaults[modelName]; ok {
		return n
	}
	// Case-insensitive prefix match.
	upper := strings.ToUpper(modelName)
	for prefix, n := range zhipuModelDefaults {
		if strings.HasPrefix(upper, strings.ToUpper(prefix)) {
			return n
		}
	}
	return 8192
}

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
