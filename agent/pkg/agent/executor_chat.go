package agent

import (
	"context"
	"fmt"

	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/model"
)

// ChatRequest is the input to Executor.Chat.
type ChatRequest struct {
	// Message is the user message to send to the supervisor agent.
	Message string `json:"message"`
	// Stream requests token-by-token streaming when true.
	// If false, the full response is buffered and returned as one string.
	Stream bool `json:"stream,omitempty"`
}

// ChatResponse is the output of a non-streaming Executor.Chat call.
type ChatResponse struct {
	// Reply is the supervisor's full text response.
	Reply string `json:"reply"`
	// InputTokens is the number of tokens in the request (best-effort).
	InputTokens int `json:"input_tokens,omitempty"`
	// OutputTokens is the number of tokens in the response (best-effort).
	OutputTokens int `json:"output_tokens,omitempty"`
}

// Chat sends message directly to the supervisor agent and returns the reply.
//
// It bypasses the autonomous ORIENT→PLAN→DISPATCH cycle and injects the
// message directly into the supervisor AgentContext, making a single
// synchronous LLM call. This is the endpoint used by interactive chat and
// the E2E test suite.
//
// The supervisor context (window + compaction) operates normally — the
// injected message and response are appended and snapshots are saved.
//
// Returns an error when the executor is not yet initialised (InitContext not
// called) or when the gateway call fails.
func (e *Executor) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if e.supervisorCtx == nil {
		return nil, fmt.Errorf("executor not initialised: call InitContext first")
	}

	// Try streaming if the gateway supports it and the caller requested it.
	if req.Stream {
		if sg, ok := e.gateway.(model.StreamingGateway); ok {
			return e.chatStream(ctx, sg, req.Message)
		}
		// Fall through to non-streaming if gateway doesn't support it.
	}

	resp, err := e.inferWithContext(ctx, e.supervisorCtx, req.Message)
	if err != nil {
		return nil, fmt.Errorf("chat infer: %w", err)
	}
	e.saveCheckpoint()
	return &ChatResponse{
		Reply:        resp.Content,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	}, nil
}

// chatStream performs a streaming chat call, collecting the full response.
func (e *Executor) chatStream(ctx context.Context, sg model.StreamingGateway, message string) (*ChatResponse, error) {
	// Append user message to context.
	if message != "" {
		m, err := e.newUserMsg(message)
		if err != nil {
			return nil, fmt.Errorf("chat stream: build user msg: %w", err)
		}
		if err := e.supervisorCtx.AppendMessage(ctx, m); err != nil {
			return nil, fmt.Errorf("chat stream: append user msg: %w", err)
		}
	}

	// Build the serialised request payload.
	adapter, err := e.adapterReg.Get(e.gateway.Provider())
	if err != nil {
		return nil, fmt.Errorf("chat stream: adapter: %w", err)
	}
	ca := llmctx.NewContextAdapter(e.supervisorCtx, adapter)
	payload, err := ca.BuildRequest(llmctx.RequestOptions{
		Model:     e.gateway.ModelName(),
		MaxTokens: e.gateway.MaxTokens(),
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
	if agtMsg, err := e.newAgentMsg(AuthorAgent, content); err == nil {
		e.supervisorCtx.AppendMessage(ctx, agtMsg)
	}
	e.saveCheckpoint()

	return &ChatResponse{Reply: content}, nil
}
