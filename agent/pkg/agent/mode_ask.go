package agent

import (
	"context"
	"fmt"

	"github.com/quarkloop/agent/pkg/model"
)

// chatAsk handles a chat request in ask mode. It performs a single LLM call
// with a read-only system prompt and returns the answer directly. No plans
// are created and no tools are invoked.
func (a *Agent) chatAsk(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Streaming path.
	if req.Stream {
		if sg, ok := a.gateway.(model.StreamingGateway); ok {
			resp, err := a.chatStream(ctx, sg, req.Message)
			if err != nil {
				return nil, err
			}
			resp.Mode = string(ModeAsk)
			if resp.Reply == "" {
				resp.Warning = "model returned an empty response"
			}
			return resp, nil
		}
	}

	resp, err := a.inferWithContext(ctx, a.ctx, req.Message)
	if err != nil {
		return nil, fmt.Errorf("ask infer: %w", err)
	}

	cr := &ChatResponse{
		Reply:        resp.Content,
		Mode:         string(ModeAsk),
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	}
	if resp.Content == "" {
		cr.Warning = "model returned an empty response"
	}

	a.saveCheckpoint()
	return cr, nil
}
