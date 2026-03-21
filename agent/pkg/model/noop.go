package model

import (
	"context"
	"fmt"
	"strings"
)

// noopGateway is a zero-dependency stub used when provider is "noop" or when
// QUARK_DRY_RUN=1 is set. It echoes back a canned response so the full
// agent/KB/tool pipeline can be exercised without a real API key.
type noopGateway struct {
	model string
}

func (g *noopGateway) Provider() string         { return "noop" }
func (g *noopGateway) ModelName() string        { return g.model }
func (g *noopGateway) MaxTokens() int           { return 4096 }
func (g *noopGateway) Parser() ToolCallParser   { return ParserFor("noop") }

func (g *noopGateway) InferRaw(_ context.Context, payload []byte) (*RawResponse, error) {
	// Echo the first 120 chars of the payload as a canned reply so tests can
	// assert the pipeline reached the gateway without needing a real API call.
	preview := string(payload)
	if len(preview) > 120 {
		preview = preview[:120] + "..."
	}
	reply := fmt.Sprintf(
		`{"role":"assistant","content":"[noop] received %d bytes: %s"}`,
		len(payload),
		strings.ReplaceAll(preview, `"`, `'`),
	)
	return &RawResponse{
		Content:      reply,
		InputTokens:  len(payload) / 4, // rough estimate
		OutputTokens: len(reply) / 4,
	}, nil
}
