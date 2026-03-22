package chat

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/inference"
)

// processAuto handles a chat request in auto mode. It uses a lightweight LLM
// call to classify the request, then routes to the resolved mode's handler.
func processAuto(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	deps *Deps,
	req agentcore.ChatRequest,
) (*agentcore.ChatResponse, error) {
	resolved, err := classifyMode(ctx, ac, res, req.Message)
	if err != nil {
		log.Printf("chat: auto classification failed: %v — falling back to plan", err)
		resolved = agentcore.ModePlan
	}

	log.Printf("chat: auto classified as %s", resolved)
	emitActivity(res.Activity, req.SessionKey, activity.ModeClassified, map[string]string{"resolved": string(resolved)})

	switch resolved {
	case agentcore.ModeAsk:
		return processAsk(ctx, ac, res, deps, req)
	case agentcore.ModePlan:
		return processPlan(ctx, ac, res, deps, req)
	case agentcore.ModeMasterPlan:
		return processMasterPlan(ctx, ac, res, deps, req)
	default:
		return processPlan(ctx, ac, res, deps, req)
	}
}

// classifyMode performs a single LLM call to determine the appropriate mode.
func classifyMode(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	message string,
) (agentcore.Mode, error) {
	prompt := ClassificationPrompt(message)
	resp, err := inference.Infer(ctx, ac, res, prompt)
	if err != nil {
		return "", fmt.Errorf("classify: %w", err)
	}
	return agentcore.ParseMode(strings.TrimSpace(resp.Content))
}
