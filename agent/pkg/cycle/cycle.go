// Package cycle isolates the supervisor ORIENT → PLAN → DISPATCH → MONITOR → ASSESS loop.
// Single responsibility: drive one iteration of the autonomous plan execution cycle.
package cycle

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/context/freshness"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/execution"
	"github.com/quarkloop/agent/pkg/inference"
	"github.com/quarkloop/agent/pkg/intervention"
	"github.com/quarkloop/agent/pkg/plan"
)

// WorkerSpawner is called by the supervisor to dispatch ready plan steps.
// The agent implements this to create sub-sessions and launch workers.
type WorkerSpawner interface {
	SpawnWorker(ctx context.Context, step plan.Step) error
}

// Supervisor runs one iteration of the autonomous loop.
// Returns true when the plan is complete.
func Supervisor(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	planStore *plan.Store,
	spawner WorkerSpawner,
	subAgents map[string]*agentcore.Definition,
	interventions *intervention.Queue,
) (bool, error) {
	scanFreshness(ctx, ac)

	// Check for interventions before orient — if present, append to context.
	if interventions != nil {
		if msgs := interventions.Poll(intervention.Drain); len(msgs) > 0 {
			for _, msg := range msgs {
				userMsg, err := inference.NewUserMessage(res.TC, res.IDGen, agentcore.AuthorUser, msg.Content)
				if err == nil {
					ac.AppendMessage(ctx, userMsg)
				}
			}
			res.EventBus.Emit(eventbus.Event{
				Kind: eventbus.KindIntervention,
				Data: map[string]string{"count": fmt.Sprintf("%d", len(msgs))},
			})
		}
	}

	state, err := orient(ac, res, planStore, subAgents)
	if err != nil {
		return false, fmt.Errorf("orient: %w", err)
	}
	if err := updatePlan(ctx, ac, res, planStore, state); err != nil {
		return false, fmt.Errorf("plan: %w", err)
	}
	if err := dispatch(ctx, res, planStore, spawner); err != nil {
		return false, fmt.Errorf("dispatch: %w", err)
	}
	if err := monitor(res, planStore); err != nil {
		return false, fmt.Errorf("monitor: %w", err)
	}
	return assess(planStore)
}

// scanFreshness runs the llmctx freshness scanner on the agent context.
func scanFreshness(ctx context.Context, ac *llmctx.AgentContext) {
	if ac == nil {
		return
	}
	vctx := freshness.ValidationContext{Now: time.Now().UTC()}
	report, err := ac.RefreshStale(ctx, vctx)
	if err != nil {
		log.Printf("cycle: freshness scan error: %v", err)
		return
	}
	if report.HasIssues() {
		log.Printf("cycle: freshness scan: %d stale (refreshed=%d, removed=%d, flagged=%d)",
			len(report.Stale), report.RefreshedCount, report.RemovedCount, report.FlaggedCount)
	}
}

// orient reads KB state and builds a concise summary for the supervisor.
func orient(
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	planStore *plan.Store,
	subAgents map[string]*agentcore.Definition,
) (map[string]interface{}, error) {
	state := map[string]interface{}{}

	if goal, err := res.KB.Get(agentcore.NSConfig, agentcore.KeyGoal); err == nil {
		state["goal"] = string(goal)
	}

	if p, err := planStore.Load(); err == nil {
		state["plan"] = p
		counts := map[string]int{"pending": 0, "running": 0, "complete": 0, "failed": 0}
		for _, s := range p.Steps {
			counts[string(s.Status)]++
		}
		state["step_counts"] = counts
	}

	eventKeys, _ := res.KB.List(agentcore.NSEvents)
	events := []string{}
	for _, k := range eventKeys {
		if data, err := res.KB.Get(agentcore.NSEvents, k); err == nil {
			events = append(events, string(data))
		}
	}
	if len(events) > 0 {
		state["recent_events"] = events
	}

	agentNames := make([]string, 0, len(subAgents))
	for name := range subAgents {
		agentNames = append(agentNames, name)
	}
	state["available_agents"] = agentNames
	state["available_tools"] = res.Dispatcher.List()

	if ac != nil {
		s := ac.Stats()
		state["context_pressure"] = string(s.Pressure)
		state["context_tokens"] = s.TotalTokens.Value()
	}

	return state, nil
}

// updatePlan calls the model gateway to produce or update the master plan.
func updatePlan(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	planStore *plan.Store,
	state map[string]interface{},
) error {
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

	resp, err := inference.Infer(ctx, ac, res, userMsg)
	if err != nil {
		return fmt.Errorf("model infer: %w", err)
	}

	log.Printf("cycle: plan response (%d tokens): %s", resp.TotalTokens(), execution.Truncate(resp.Content, 200))

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
		existing, err := planStore.Load()
		if err != nil {
			log.Printf("cycle: plan load error during completion: %v", err)
		}
		if existing == nil {
			existing = &plan.Plan{}
		}
		existing.Complete = true
		existing.Summary = check.Summary
		existing.UpdatedAt = time.Now()
		return planStore.Save(existing)
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

	existing, _ := planStore.Load()
	existingStatus := map[string]plan.Step{}
	planStatus := plan.PlanDraft
	if existing != nil {
		if existing.Status != "" {
			planStatus = existing.Status
		}
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
		Status:    planStatus,
		Steps:     mergedSteps,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing != nil {
		p.CreatedAt = existing.CreatedAt
	}
	return planStore.Save(p)
}

// dispatch reads the current plan and spawns workers for ready steps.
func dispatch(ctx context.Context, res *agentcore.Resources, planStore *plan.Store, spawner WorkerSpawner) error {
	p, err := planStore.Load()
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

		now := time.Now()
		step.Status = plan.StepRunning
		step.StartedAt = &now
		if err := planStore.Save(p); err != nil {
			return err
		}
		log.Printf("cycle: dispatching step %s to agent %s", step.ID, step.Agent)
		emitActivity(res.EventBus, eventbus.KindStepDispatched, map[string]string{"step": step.ID, "agent": step.Agent})

		if err := spawner.SpawnWorker(ctx, *step); err != nil {
			log.Printf("cycle: spawn worker error for step %s: %v", step.ID, err)
		}
	}
	return nil
}

// monitor reads completion events from the KB and updates the plan.
func monitor(res *agentcore.Resources, planStore *plan.Store) error {
	p, err := planStore.Load()
	if err != nil || p == nil {
		return nil
	}

	eventKeys, err := res.KB.List(agentcore.NSEvents)
	if err != nil {
		return nil
	}

	modified := false
	for _, key := range eventKeys {
		data, err := res.KB.Get(agentcore.NSEvents, key)
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
				res.KB.Delete(agentcore.NSEvents, key)
				modified = true
				if event.Status == "complete" {
					emitActivity(res.EventBus, eventbus.KindStepCompleted, map[string]string{"step": event.StepID})
				} else {
					emitActivity(res.EventBus, eventbus.KindStepFailed, map[string]string{"step": event.StepID, "error": event.Error})
				}
				log.Printf("cycle: step %s → %s", event.StepID, event.Status)
				break
			}
		}
	}

	if modified {
		return planStore.Save(p)
	}
	return nil
}

// assess checks whether the goal is fully achieved.
func assess(planStore *plan.Store) (bool, error) {
	p, err := planStore.Load()
	if err != nil || p == nil {
		return false, nil
	}
	if p.Complete {
		log.Printf("cycle: plan complete — %s", p.Summary)
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
			planStore.Save(p)
			return true, nil
		}
	}
	return false, nil
}

func emitActivity(bus *eventbus.Bus, kind eventbus.EventKind, data interface{}) {
	if bus == nil {
		return
	}
	bus.Emit(eventbus.Event{
		Kind: kind,
		Data: data,
	})
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
