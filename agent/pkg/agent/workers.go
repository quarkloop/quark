package agent

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/quarkloop/agent/pkg/agentcore"
	"github.com/quarkloop/agent/pkg/execution"
	"github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/agent/pkg/session"
)

// SpawnWorker implements cycle.WorkerSpawner.
// It creates a sub-agent session and launches a worker goroutine.
func (a *Agent) SpawnWorker(ctx context.Context, step plan.Step) error {
	sessKey := session.SubAgentKey(a.agentID, step.ID)
	now := time.Now()
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
	_ = a.sessStore.Create(sess)

	go a.runWorker(ctx, step, sessKey)
	return nil
}

func (a *Agent) runWorker(ctx context.Context, step plan.Step, sessKey string) {
	log.Printf("worker[%s]: starting (%s)", step.ID, step.Agent)

	artifacts := execution.GatherArtifacts(a.res)
	result, err := execution.ExecuteStep(ctx, a.res, a.def, step, a.subAgents, artifacts)

	event := map[string]string{
		"step_id": step.ID,
		"status":  "complete",
		"result":  result,
	}
	if err != nil {
		log.Printf("worker[%s]: failed: %v", step.ID, err)
		event["status"] = "failed"
		event["error"] = err.Error()
	} else {
		log.Printf("worker[%s]: complete", step.ID)
	}

	data, _ := json.Marshal(event)
	if err := a.res.KB.Set(agentcore.NSEvents, step.ID+"-done", data); err != nil {
		log.Printf("worker[%s]: failed to write event: %v", step.ID, err)
	}

	// Mark sub-agent session complete.
	now := time.Now()
	if sess, err := a.sessStore.Get(sessKey); err == nil {
		sess.Status = session.StatusCompleted
		sess.EndedAt = &now
		sess.UpdatedAt = now
		a.sessStore.Update(sess)
	}
}
