package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/context/freshness"
	"github.com/quarkloop/agent/pkg/plan"
)

// supervisorCycle executes one ORIENT → PLAN → DISPATCH → MONITOR → ASSESS pass.
func (a *Agent) supervisorCycle(ctx context.Context) (bool, error) {
	a.scanFreshness(ctx)

	state, err := a.orient(ctx)
	if err != nil {
		return false, fmt.Errorf("orient: %w", err)
	}
	if err := a.updatePlan(ctx, state); err != nil {
		return false, fmt.Errorf("plan: %w", err)
	}
	if err := a.dispatch(ctx); err != nil {
		return false, fmt.Errorf("dispatch: %w", err)
	}
	if err := a.monitor(ctx); err != nil {
		return false, fmt.Errorf("monitor: %w", err)
	}
	return a.assess(ctx)
}

// ─── FRESHNESS ───────────────────────────────────────────────────────────────

// scanFreshness runs the llmctx freshness scanner on the agent context,
// removing stale messages that cannot be refreshed.
func (a *Agent) scanFreshness(ctx context.Context) {
	if a.ctx == nil {
		return
	}
	vctx := freshness.ValidationContext{Now: time.Now().UTC()}
	report, err := a.ctx.RefreshStale(ctx, vctx)
	if err != nil {
		log.Printf("agent: freshness scan error: %v", err)
		return
	}
	if report.HasIssues() {
		log.Printf("agent: freshness scan: %d stale (refreshed=%d, removed=%d, flagged=%d)",
			len(report.Stale), report.RefreshedCount, report.RemovedCount, report.FlaggedCount)
	}
}

// ─── ORIENT ──────────────────────────────────────────────────────────────────

// orient reads KB state and builds a concise summary for the supervisor.
func (a *Agent) orient(_ context.Context) (map[string]interface{}, error) {
	state := map[string]interface{}{}

	if goal, err := a.kb.Get(NSConfig, KeyGoal); err == nil {
		state["goal"] = string(goal)
	}

	if p, err := a.planStore.Load(); err == nil {
		state["plan"] = p
		counts := map[string]int{"pending": 0, "running": 0, "complete": 0, "failed": 0}
		for _, s := range p.Steps {
			counts[string(s.Status)]++
		}
		state["step_counts"] = counts
	}

	eventKeys, _ := a.kb.List(NSEvents)
	events := []string{}
	for _, k := range eventKeys {
		if data, err := a.kb.Get(NSEvents, k); err == nil {
			events = append(events, string(data))
		}
	}
	if len(events) > 0 {
		state["recent_events"] = events
	}

	agentNames := make([]string, 0, len(a.subAgents))
	for name := range a.subAgents {
		agentNames = append(agentNames, name)
	}
	state["available_agents"] = agentNames
	state["available_skills"] = a.dispatcher.List()

	if a.ctx != nil {
		s := a.ctx.Stats()
		state["context_pressure"] = string(s.Pressure)
		state["context_tokens"] = s.TotalTokens.Value()
	}

	return state, nil
}

// ─── PLAN ────────────────────────────────────────────────────────────────────

// updatePlan calls the model gateway to produce or update the master plan.
func (a *Agent) updatePlan(ctx context.Context, state map[string]interface{}) error {
	if p, ok := state["plan"].(*plan.Plan); ok {
		if p.Complete {
			return nil
		}
		allSettled := true
		for _, s := range p.Steps {
			if s.Status == plan.StepPending {
				allSettled = false
				break
			}
		}
		if allSettled {
			return nil
		}
	}

	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	userMsg := fmt.Sprintf(`Current state:
%s

Produce or update the execution plan as a JSON object with this exact structure:
{
  "goal": "<restate the goal concisely>",
  "steps": [
    {
      "id": "<short-slug>",
      "agent": "<agent-name from available_agents, or 'supervisor'>",
      "description": "<specific task for this agent>",
      "depends_on": ["<step-id>", ...]
    }
  ]
}

Rules:
- Each step must have a unique "id" (e.g. "research-1", "write-draft").
- "agent" must be one of the available_agents or "supervisor".
- "depends_on" lists step IDs that must complete before this step can start.
- Keep steps focused and atomic.
- If the goal is already fully achieved, return {"complete": true, "summary": "<what was done>"}.
- Respond with ONLY the JSON object, no explanation.`, string(stateJSON))

	resp, err := a.inferWithContext(ctx, a.ctx, userMsg)
	if err != nil {
		return fmt.Errorf("model infer: %w", err)
	}

	log.Printf("agent: plan response (%d tokens): %s", resp.TotalTokens(), truncate(resp.Content, 200))

	planData, err := extractJSON(resp.Content)
	if err != nil {
		return fmt.Errorf("extracting plan JSON: %w", err)
	}

	// Handle completion signal
	var check struct {
		Complete bool   `json:"complete"`
		Summary  string `json:"summary"`
	}
	if err := json.Unmarshal(planData, &check); err == nil && check.Complete {
		existing, _ := a.planStore.Load()
		if existing == nil {
			existing = &plan.Plan{}
		}
		existing.Complete = true
		existing.Summary = check.Summary
		existing.UpdatedAt = time.Now()
		return a.planStore.Save(existing)
	}

	// Parse new/updated plan
	var newPlan struct {
		Goal  string `json:"goal"`
		Steps []struct {
			ID          string   `json:"id"`
			Agent       string   `json:"agent"`
			Description string   `json:"description"`
			DependsOn   []string `json:"depends_on"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(planData, &newPlan); err != nil {
		return fmt.Errorf("parsing plan: %w", err)
	}

	// Merge with existing plan — preserve status of steps we already know about.
	existing, _ := a.planStore.Load()
	existingStatus := map[string]plan.Step{}
	if existing != nil {
		for _, s := range existing.Steps {
			existingStatus[s.ID] = s
		}
	}

	now := time.Now()
	mergedSteps := make([]plan.Step, 0, len(newPlan.Steps))
	for _, ns := range newPlan.Steps {
		step := plan.Step{
			ID:          ns.ID,
			Agent:       ns.Agent,
			Description: ns.Description,
			DependsOn:   ns.DependsOn,
			Status:      plan.StepPending,
		}
		if prev, ok := existingStatus[ns.ID]; ok {
			step.Status = prev.Status
			step.Result = prev.Result
			step.Error = prev.Error
			step.StartedAt = prev.StartedAt
			step.FinishedAt = prev.FinishedAt
		}
		mergedSteps = append(mergedSteps, step)
	}

	p := &plan.Plan{
		Goal:      newPlan.Goal,
		Steps:     mergedSteps,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing != nil {
		p.CreatedAt = existing.CreatedAt
	}
	return a.planStore.Save(p)
}

// ─── DISPATCH ────────────────────────────────────────────────────────────────

// dispatch reads the current plan and spawns goroutines for ready steps.
func (a *Agent) dispatch(ctx context.Context) error {
	p, err := a.planStore.Load()
	if err != nil || p == nil || p.Complete {
		return nil
	}

	for i := range p.Steps {
		step := &p.Steps[i]
		if step.Status != plan.StepPending {
			continue
		}
		if !plan.DepsComplete(p, step) {
			continue
		}
		a.mu.Lock()
		_, alreadyActive := a.activeSteps[step.ID]
		if !alreadyActive {
			a.activeSteps[step.ID] = struct{}{}
		}
		a.mu.Unlock()
		if alreadyActive {
			continue
		}

		now := time.Now()
		step.Status = plan.StepRunning
		step.StartedAt = &now
		if err := a.planStore.Save(p); err != nil {
			return err
		}
		log.Printf("agent: dispatching step %s to agent %s", step.ID, step.Agent)
		a.emit(activity.StepDispatched, map[string]string{"step": step.ID, "agent": step.Agent})
		go a.runWorker(ctx, *step)
	}
	return nil
}

// ─── MONITOR ─────────────────────────────────────────────────────────────────

// monitor reads completion events from the KB and updates the plan accordingly.
func (a *Agent) monitor(_ context.Context) error {
	p, err := a.planStore.Load()
	if err != nil || p == nil {
		return nil
	}

	eventKeys, err := a.kb.List(NSEvents)
	if err != nil {
		return nil
	}

	modified := false
	for _, key := range eventKeys {
		data, err := a.kb.Get(NSEvents, key)
		if err != nil {
			continue
		}
		var event struct {
			StepID string `json:"step_id"`
			Status string `json:"status"`
			Result string `json:"result"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}
		for i := range p.Steps {
			if p.Steps[i].ID == event.StepID {
				now := time.Now()
				p.Steps[i].FinishedAt = &now
				p.Steps[i].Result = event.Result
				p.Steps[i].Error = event.Error
				if event.Status == "complete" {
					p.Steps[i].Status = plan.StepComplete
				} else {
					p.Steps[i].Status = plan.StepFailed
				}
				a.mu.Lock()
				delete(a.activeSteps, event.StepID)
				a.mu.Unlock()
				a.kb.Delete(NSEvents, key)
				modified = true
				if event.Status == "complete" {
					a.emit(activity.StepCompleted, map[string]string{"step": event.StepID})
				} else {
					a.emit(activity.StepFailed, map[string]string{"step": event.StepID, "error": event.Error})
				}
				log.Printf("agent: step %s → %s", event.StepID, event.Status)
				break
			}
		}
	}

	if modified {
		return a.planStore.Save(p)
	}
	return nil
}

// ─── ASSESS ──────────────────────────────────────────────────────────────────

// assess checks whether the goal is fully achieved.
func (a *Agent) assess(_ context.Context) (bool, error) {
	p, err := a.planStore.Load()
	if err != nil || p == nil {
		return false, nil
	}
	if p.Complete {
		log.Printf("agent: plan complete — %s", p.Summary)
		return true, nil
	}
	if len(p.Steps) > 0 {
		allDone := true
		for _, s := range p.Steps {
			if s.Status != plan.StepComplete {
				allDone = false
				break
			}
		}
		if allDone {
			p.Complete = true
			p.Summary = "All steps completed successfully."
			p.UpdatedAt = time.Now()
			a.planStore.Save(p)
			return true, nil
		}
	}
	return false, nil
}
