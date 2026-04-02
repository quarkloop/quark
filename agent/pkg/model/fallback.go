package model

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
)

// isRetryableError returns true for transient errors that warrant trying
// the next gateway in the fallback chain.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Network-level timeouts are always retryable.
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	msg := err.Error()
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") {
		return true
	}
	if strings.Contains(msg, "500") || strings.Contains(msg, "502") || strings.Contains(msg, "503") {
		return true
	}
	if strings.Contains(msg, "connection refused") {
		return true
	}
	return false
}

// FallbackGateway tries gateways in order until one succeeds.
type FallbackGateway struct {
	chain []Gateway
}

// NewFallbackGateway creates a fallback gateway with a primary and optional fallbacks.
func NewFallbackGateway(primary Gateway, fallbacks ...Gateway) *FallbackGateway {
	chain := make([]Gateway, 0, 1+len(fallbacks))
	chain = append(chain, primary)
	chain = append(chain, fallbacks...)
	return &FallbackGateway{chain: chain}
}

func (f *FallbackGateway) InferRaw(ctx context.Context, payload []byte) (*RawResponse, error) {
	for i, gw := range f.chain {
		resp, err := gw.InferRaw(ctx, payload)
		if err == nil {
			return resp, nil
		}
		if !isRetryableError(err) {
			return nil, err
		}
		log.Printf("model: gateway %d (%s/%s) failed: %v — trying next",
			i, gw.Provider(), gw.ModelName(), err)
	}
	return nil, fmt.Errorf("all gateways in fallback chain failed")
}

func (f *FallbackGateway) Provider() string {
	if len(f.chain) > 0 {
		return f.chain[0].Provider()
	}
	return ""
}

func (f *FallbackGateway) ModelName() string {
	if len(f.chain) > 0 {
		return f.chain[0].ModelName()
	}
	return ""
}

func (f *FallbackGateway) MaxTokens() int {
	if len(f.chain) > 0 {
		return f.chain[0].MaxTokens()
	}
	return 0
}

func (f *FallbackGateway) Parser() ToolCallParser {
	if len(f.chain) > 0 {
		return f.chain[0].Parser()
	}
	return nil
}
