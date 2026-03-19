// Package model abstracts the LLM provider layer behind the Gateway interface.
//
// The Executor calls Gateway.InferRaw with a pre-serialised JSON payload
// produced by the llmctx ContextAdapter — so this package is provider-specific
// only in its auth, transport, and response-parsing logic.
//
// Supported providers:
//
//	"anthropic"  — claude-*  (ANTHROPIC_API_KEY required)
//	"openai"     — gpt-*    (OPENAI_API_KEY required)
//	"zhipu"      — glm-*    (ZHIPU_API_KEY required)
//	"openrouter" — any model via OpenRouter (OPENROUTER_API_KEY required)
//	"noop"       — echo stub, no key required; activated by QUARK_DRY_RUN=1
package model

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

const openRouterBaseURL = "https://openrouter.ai/api/v1/chat/completions"

// RawResponse is the normalised output from a single LLM inference call.
// All provider-specific shapes are reduced to this before returning to the
// Executor.
type RawResponse struct {
	Content      string `json:"content"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

func (r *RawResponse) TotalTokens() int { return r.InputTokens + r.OutputTokens }

// Gateway abstracts LLM provider HTTP transport.
// It accepts a pre-built request payload (from ContextAdapter) and
// handles authentication, HTTP transport, and response parsing.
type Gateway interface {
	// InferRaw sends a pre-serialized request payload to the provider API
	// and returns the parsed response. The payload is built by ContextAdapter.
	InferRaw(ctx context.Context, payload []byte) (*RawResponse, error)

	// Provider returns the provider identifier (e.g. "anthropic", "openai").
	Provider() string

	// ModelName returns the configured model identifier.
	ModelName() string

	// MaxTokens returns the maximum output tokens for this gateway.
	MaxTokens() int
}

// GatewayConfig carries the constructor arguments for New.
// APIKey is ignored when Provider is "noop".
type GatewayConfig struct {
	Provider string
	Model    string
	APIKey   string
}

// New creates a Gateway for cfg.Provider.
// Returns an error when an API key is required but empty, or the provider
// string is not recognised.
func New(cfg GatewayConfig) (Gateway, error) {
	switch cfg.Provider {
	case "anthropic":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("anthropic api key not provided")
		}
		return &anthropicGateway{model: cfg.Model, apiKey: cfg.APIKey, http: newHTTPClient()}, nil
	case "openai":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openai api key not provided")
		}
		return &openAIGateway{provider: "openai", model: cfg.Model, apiKey: cfg.APIKey, http: newHTTPClient()}, nil
	case "zhipu":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("zhipu api key not provided")
		}
		return &zhipuGateway{model: cfg.Model, apiKey: cfg.APIKey, http: newHTTPClientNoHTTP2()}, nil
	case "openrouter":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openrouter api key not provided")
		}
		return &openAIGateway{
			provider: "openrouter",
			baseURL:  openRouterBaseURL,
			model:    cfg.Model,
			apiKey:   cfg.APIKey,
			http:     newHTTPClient(),
		}, nil
	case "noop":
		// Dry-run stub: no API key required. Also activated when QUARK_DRY_RUN=1
		// regardless of provider, so any Quarkfile can be tested without credentials.
		modelName := cfg.Model
		if modelName == "" {
			modelName = "noop"
		}
		return &noopGateway{model: modelName}, nil
	default:
		return nil, fmt.Errorf("unsupported model provider %q (supported: anthropic, openai, zhipu, openrouter)", cfg.Provider)
	}
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 120 * time.Second}
}

// newHTTPClientNoHTTP2 returns an HTTP client with HTTP/2 disabled.
// Zhipu's API stalls HTTP/2 connections on rate-limit instead of returning 429.
func newHTTPClientNoHTTP2() *http.Client {
	return &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			ForceAttemptHTTP2: false,
			TLSNextProto:     make(map[string]func(string, *tls.Conn) http.RoundTripper),
		},
	}
}
