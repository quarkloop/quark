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
	// Mode optionally sets the working mode for this request.
	// Valid values: "ask", "plan", "masterplan", "auto".
	// When empty, the agent uses its current mode (default: auto).
	Mode string `json:"mode,omitempty"`
}

// ChatResponse is the output of a non-streaming Agent.Chat call.
type ChatResponse struct {
	// Reply is the agent's full text response.
	Reply string `json:"reply"`
	// Mode is the resolved working mode used for this request.
	Mode string `json:"mode,omitempty"`
	// Warning is set when the response could not be fully processed
	// (e.g. plan JSON extraction failed, empty LLM response).
	Warning string `json:"warning,omitempty"`
	// InputTokens is the number of tokens in the request (best-effort).
	InputTokens int `json:"input_tokens,omitempty"`
	// OutputTokens is the number of tokens in the response (best-effort).
	OutputTokens int `json:"output_tokens,omitempty"`
}

// Chat sends a message to the agent and returns the reply. The processing
// strategy is determined by the working mode specified in the request (or
// the agent's current mode when the request omits it):
//
//   - ask:        single LLM call, read-only, no plans or tools.
//   - plan:       creates a single execution plan and returns it.
//   - masterplan: creates a master plan with phases and returns it.
//   - auto:       classifies the request via LLM, then routes to the
//     appropriate mode.
//
// The agent context (window + compaction) operates normally — messages and
// responses are appended and snapshots are saved.
func (a *Agent) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("agent not initialised: call InitContext first")
	}

	mode := a.resolveMode(req.Mode)
	switch mode {
	case ModeAsk:
		return a.chatAsk(ctx, req)
	case ModePlan:
		return a.chatPlan(ctx, req)
	case ModeMasterPlan:
		return a.chatMasterPlan(ctx, req)
	case ModeAuto:
		return a.chatAuto(ctx, req)
	default:
		return a.chatAsk(ctx, req)
	}
}

// resolveMode returns the mode to use for this request. When the request
// specifies a valid mode, the agent's current mode is updated. Otherwise
// the agent's stored mode is used.
func (a *Agent) resolveMode(reqMode string) Mode {
	if reqMode != "" {
		if m, err := ParseMode(reqMode); err == nil {
			a.SetMode(m)
			return m
		}
	}
	return a.mode
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
