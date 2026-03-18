package agent

import (
	"context"
	"fmt"

	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/model"
)

// ChatRequest is the input to Agent.Chat.
type ChatRequest struct {
	// Message is the user message to send to the agent.
	Message string `json:"message"`
	// Stream requests token-by-token streaming when true.
	// If false, the full response is buffered and returned as one string.
	Stream bool `json:"stream,omitempty"`
}

// ChatResponse is the output of a non-streaming Agent.Chat call.
type ChatResponse struct {
	// Reply is the agent's full text response.
	Reply string `json:"reply"`
	// InputTokens is the number of tokens in the request (best-effort).
	InputTokens int `json:"input_tokens,omitempty"`
	// OutputTokens is the number of tokens in the response (best-effort).
	OutputTokens int `json:"output_tokens,omitempty"`
}

// Chat sends message directly to the agent and returns the reply.
//
// It bypasses the autonomous ORIENT→PLAN→DISPATCH cycle and injects the
// message directly into the AgentContext, making a single synchronous LLM
// call. This is the endpoint used by interactive chat and the E2E test suite.
//
// The agent context (window + compaction) operates normally — the injected
// message and response are appended and snapshots are saved.
//
// Returns an error when the agent is not yet initialised (InitContext not
// called) or when the gateway call fails.
func (a *Agent) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("agent not initialised: call InitContext first")
	}

	// Try streaming if the gateway supports it and the caller requested it.
	if req.Stream {
		if sg, ok := a.gateway.(model.StreamingGateway); ok {
			return a.chatStream(ctx, sg, req.Message)
		}
		// Fall through to non-streaming if gateway doesn't support it.
	}

	resp, err := a.inferWithContext(ctx, a.ctx, req.Message)
	if err != nil {
		return nil, fmt.Errorf("chat infer: %w", err)
	}
	a.saveCheckpoint()
	return &ChatResponse{
		Reply:        resp.Content,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	}, nil
}

// chatStream performs a streaming chat call, collecting the full response.
func (a *Agent) chatStream(ctx context.Context, sg model.StreamingGateway, message string) (*ChatResponse, error) {
	// Append user message to context.
	if message != "" {
		m, err := a.newUserMsg(message)
		if err != nil {
			return nil, fmt.Errorf("chat stream: build user msg: %w", err)
		}
		if err := a.ctx.AppendMessage(ctx, m); err != nil {
			return nil, fmt.Errorf("chat stream: append user msg: %w", err)
		}
	}

	// Build the serialised request payload.
	adapter, err := a.adapterReg.Get(a.gateway.Provider())
	if err != nil {
		return nil, fmt.Errorf("chat stream: adapter: %w", err)
	}
	ca := llmctx.NewContextAdapter(a.ctx, adapter)
	payload, err := ca.BuildRequest(llmctx.RequestOptions{
		Model:     a.gateway.ModelName(),
		MaxTokens: a.gateway.MaxTokens(),
	})
	if err != nil {
		return nil, fmt.Errorf("chat stream: build request: %w", err)
	}

	ch, err := sg.InferStream(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("chat stream: infer: %w", err)
	}

	content, err := model.CollectStream(ch)
	if err != nil {
		return nil, fmt.Errorf("chat stream: collect: %w", err)
	}

	// Append the assistant response to context.
	if agtMsg, err := a.newAgentMsg(AuthorAgent, content); err == nil {
		a.ctx.AppendMessage(ctx, agtMsg)
	}
	a.saveCheckpoint()

	return &ChatResponse{Reply: content}, nil
}
