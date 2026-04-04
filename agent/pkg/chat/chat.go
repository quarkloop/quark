// Package chat isolates chat mode routing and processing.
// Single responsibility: given a context and a user message, process it
// according to the active mode (ask, plan, masterplan, auto).
package chat

import (
	"context"

	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/intervention"
	"github.com/quarkloop/agent/pkg/plan"
)

// Deps holds the dependencies that chat mode handlers need beyond Resources.
type Deps struct {
	Def           *agentcore.Definition
	SubAgents     map[string]*agentcore.Definition
	PlanStore     *plan.Store
	MasterStore   *plan.MasterPlanStore
	Interventions *intervention.Queue
}

// Process routes a chat request to the appropriate mode handler.
func Process(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	deps *Deps,
	mode agentcore.Mode,
	req agentcore.ChatRequest,
) (*agentcore.ChatResponse, error) {
	switch mode {
	case agentcore.ModeAsk:
		return processAsk(ctx, ac, res, deps, req)
	case agentcore.ModePlan:
		return processPlan(ctx, ac, res, deps, req)
	case agentcore.ModeMasterPlan:
		return processMasterPlan(ctx, ac, res, deps, req)
	case agentcore.ModeAuto:
		return processAuto(ctx, ac, res, deps, req)
	default:
		return processAsk(ctx, ac, res, deps, req)
	}
}
