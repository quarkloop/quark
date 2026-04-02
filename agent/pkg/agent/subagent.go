package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/quarkloop/agent/pkg/agentcore"
	"github.com/quarkloop/agent/pkg/execution"
	"github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/agent/pkg/session"
	"github.com/quarkloop/agent/pkg/subagent"
)

func (a *Agent) clock() time.Time { return time.Now() }

// SpawnSubagent implements cycle.SubagentSpawner.
// It delegates to the subagent manager which enforces resource boundaries.
func (a *Agent) SpawnSubagent(ctx context.Context, step plan.Step) error {
	if a.subagentMgr == nil {
		return fmt.Errorf("subagent manager not initialized")
	}

	sessKey := session.SubAgentKey(a.agentID, step.ID)
	now := a.clock()
	sess := &session.Session{
		Key:       sessKey,
		AgentID:   a.agentID,
		Type:      session.TypeSubAgent,
		Status:    session.StatusActive,
		Title:     step.Description,
		ParentKey: session.MainKey(a.agentID),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := a.sessStore.Create(sess); err != nil {
		return fmt.Errorf("create subagent session: %w", err)
	}

	tokenBudget := agentcore.DefaultContextWindow / 4

	cfg := subagent.Config{
		MaxDepth:      3,
		MaxConcurrent: a.def.Capabilities.MaxWorkers,
		TokenBudget:   tokenBudget,
		Timeout:       0, // uses default 5 min
		MaxMessages:   50,
	}

	_, err := a.subagentMgr.Spawn(ctx, a.agentID, 0, cfg, step, func(runCtx context.Context, s plan.Step, tb int, mm int) (string, error) {
		artifacts := execution.GatherArtifacts(a.res)
		return execution.ExecuteStep(runCtx, a.res, a.def, s, a.subAgents, artifacts)
	})
	return err
}
