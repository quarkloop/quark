// Package inference isolates LLM calling logic.
// Single responsibility: send messages to LLM, get responses.
package inference

import (
	"context"
	"fmt"
	"log"

	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/model"
)

// Infer appends userMsg to the context, calls the LLM gateway,
// appends the assistant response, and auto-compacts if pressure is high.
func Infer(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	userMsg string,
) (*model.RawResponse, error) {
	if userMsg != "" {
		m, err := NewUserMessage(res.TC, res.IDGen, agentcore.AuthorUser, userMsg)
		if err != nil {
			return nil, fmt.Errorf("build user msg: %w", err)
		}
		if err := ac.AppendMessage(ctx, m); err != nil {
			return nil, fmt.Errorf("append user msg: %w", err)
		}
	}

	gw := res.GetGateway()
	adapter, err := res.AdapterReg.Get(gw.Provider())
	if err != nil {
		return nil, fmt.Errorf("adapter for %s: %w", gw.Provider(), err)
	}
	ca := llmctx.NewContextAdapter(ac, adapter)

	payload, err := ca.BuildRequest(llmctx.RequestOptions{
		Model:     gw.ModelName(),
		MaxTokens: gw.MaxTokens(),
	})
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	// The gateway handles retries and fallback chains internally.
	resp, err := gw.InferRaw(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("gateway: %w", err)
	}

	agtMsg, err := NewAgentMessage(res.TC, res.IDGen, agentcore.AuthorAgent, resp.Content)
	if err == nil {
		ac.AppendMessage(ctx, agtMsg)
	}

	status := ac.BudgetStatus()
	if status.CompactionNeeded {
		log.Printf("inference: budget soft limit reached (%d/%d tokens, %.1f%%) — compacting",
			status.UsedTokens, status.TotalBudget, status.UsagePct)
		if err := ac.Compact(ctx); err != nil {
			log.Printf("inference: compact error: %v", err)
		} else if res.EventBus != nil {
			res.EventBus.Emit(eventbus.Event{
				Kind: eventbus.KindBudgetCompacted,
				Data: status,
			})
		}
	}

	return resp, nil
}

// InferWithRetry wraps Infer — retry logic is now handled by FallbackGateway.
func InferWithRetry(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	userMsg string,
) (*model.RawResponse, error) {
	return Infer(ctx, ac, res, userMsg)
}
