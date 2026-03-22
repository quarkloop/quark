// Package inference isolates LLM calling logic.
// Single responsibility: send messages to LLM, get responses.
package inference

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
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

	adapter, err := res.AdapterReg.Get(res.Gateway.Provider())
	if err != nil {
		return nil, fmt.Errorf("adapter for %s: %w", res.Gateway.Provider(), err)
	}
	ca := llmctx.NewContextAdapter(ac, adapter)

	payload, err := ca.BuildRequest(llmctx.RequestOptions{
		Model:     res.Gateway.ModelName(),
		MaxTokens: res.Gateway.MaxTokens(),
	})
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := res.Gateway.InferRaw(ctx, payload)
	if err != nil {
		resp, err = inferWithRetry(ctx, res.Gateway, payload, err)
		if err != nil {
			return nil, fmt.Errorf("gateway: %w", err)
		}
	}

	agtMsg, err := NewAgentMessage(res.TC, res.IDGen, agentcore.AuthorAgent, resp.Content)
	if err == nil {
		ac.AppendMessage(ctx, agtMsg)
	}

	if ac.Pressure() >= llmctx.PressureHigh {
		log.Printf("inference: context pressure %s — auto-compacting", ac.Pressure())
		if err := ac.Compact(ctx); err != nil {
			log.Printf("inference: compact error: %v", err)
		}
	}

	return resp, nil
}

// InferWithRetry wraps Infer with retries for transient errors (429, rate limits).
// Max 2 retries with exponential backoff (2s, 4s).
func InferWithRetry(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	userMsg string,
) (*model.RawResponse, error) {
	return Infer(ctx, ac, res, userMsg)
}

func inferWithRetry(ctx context.Context, gw model.Gateway, payload []byte, firstErr error) (*model.RawResponse, error) {
	err := firstErr
	backoff := 2 * time.Second
	for attempt := 1; attempt <= 2; attempt++ {
		if !isRetryableGatewayError(err) {
			return nil, err
		}
		log.Printf("inference: retrying transient gateway error (attempt %d/2): %v", attempt, err)
		if sleepErr := sleepWithContext(ctx, backoff); sleepErr != nil {
			return nil, err
		}

		resp, retryErr := gw.InferRaw(ctx, payload)
		if retryErr == nil {
			return resp, nil
		}
		err = retryErr
		backoff *= 2
	}
	return nil, err
}

func isRetryableGatewayError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, " 429") ||
		strings.Contains(msg, "rate-limit") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "temporarily rate-limited")
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
