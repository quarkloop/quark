// Package chat isolates chat mode routing and processing.
// Single responsibility: given a context and a user message, process it
// according to the active mode (ask, plan, masterplan, auto).
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

// extractJSON pulls the first JSON object from a string (handles markdown fences).
func extractJSON(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		end := strings.LastIndex(s, "```")
		if end > 3 {
			s = strings.TrimSpace(s[3:end])
			if nl := strings.Index(s, "\n"); nl >= 0 {
				s = strings.TrimSpace(s[nl:])
			}
		}
	}
	start := strings.Index(s, "{")
	if start < 0 {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				raw := []byte(s[start : i+1])
				if json.Valid(raw) {
					return raw, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("malformed JSON in response")
}
