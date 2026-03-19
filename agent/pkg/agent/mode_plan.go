package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/plan"
)

// chatPlan handles a chat request in plan mode. It calls the LLM with a
// plan-specific system prompt to produce or update an execution plan, stores
// the plan in the KB, and returns it to the user. The supervisor LLM decides
// how to execute the plan — this handler does not drive execution.
func (a *Agent) chatPlan(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Inject existing plan context so the LLM can see current state.
	message := a.withPlanContext(req.Message)

	var cr *ChatResponse

	// Streaming path.
	if req.Stream {
		if sg, ok := a.gateway.(model.StreamingGateway); ok {
			resp, err := a.chatStream(ctx, sg, message)
			if err != nil {
				return nil, fmt.Errorf("plan stream: %w", err)
			}
			cr = resp
			cr.Mode = string(ModePlan)
		}
	}

	// Non-streaming path.
	if cr == nil {
		resp, err := a.inferWithContext(ctx, a.ctx, message)
		if err != nil {
			return nil, fmt.Errorf("plan infer: %w", err)
		}
		cr = &ChatResponse{
			Reply:        resp.Content,
			Mode:         string(ModePlan),
			InputTokens:  resp.InputTokens,
			OutputTokens: resp.OutputTokens,
		}
	}

	// Empty response check.
	if cr.Reply == "" {
		cr.Warning = "model returned an empty response — plan was not created"
		a.saveCheckpoint()
		return cr, nil
	}

	// Try to extract and store the plan.
	planData, err := extractJSON(cr.Reply)
	if err != nil {
		cr.Warning = fmt.Sprintf("plan was not stored: %v", err)
		log.Printf("agent: plan JSON extraction failed: %v", err)
	} else if err := a.storePlanFromResponse(planData); err != nil {
		cr.Warning = fmt.Sprintf("plan was not stored: %v", err)
		log.Printf("agent: failed to store plan: %v", err)
	} else {
		a.emit(activity.PlanCreated, map[string]string{"mode": "plan"})
	}

	a.saveCheckpoint()
	return cr, nil
}

// withPlanContext prepends existing plan state to the user message so the LLM
// can see the current plan when the user sends follow-up messages (e.g. to
// approve or modify the plan).
func (a *Agent) withPlanContext(message string) string {
	existing, err := a.planStore.Load()
	if err != nil || existing == nil || existing.Complete {
		return message
	}
	planJSON, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return message
	}
	return fmt.Sprintf("Current plan:\n```json\n%s\n```\n\n%s", string(planJSON), message)
}

// storePlanFromResponse parses a JSON plan response and persists it to KB.
func (a *Agent) storePlanFromResponse(data []byte) error {
	var raw struct {
		Goal   string `json:"goal"`
		Status string `json:"status"` // "draft" or "approved"
		Steps  []struct {
			ID          string   `json:"id"`
			Agent       string   `json:"agent"`
			Description string   `json:"description"`
			DependsOn   []string `json:"depends_on"`
		} `json:"steps"`
		Complete bool   `json:"complete"`
		Summary  string `json:"summary"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing plan: %w", err)
	}

	// Handle completion signal.
	if raw.Complete {
		existing, _ := a.planStore.Load()
		if existing == nil {
			existing = &plan.Plan{}
		}
		existing.Complete = true
		existing.Summary = raw.Summary
		existing.UpdatedAt = time.Now()
		return a.planStore.Save(existing)
	}

	if len(raw.Steps) == 0 {
		return fmt.Errorf("plan has no steps")
	}

	// Resolve plan status: auto-approve if approval policy says so,
	// otherwise respect whatever the LLM returned.
	planStatus := plan.PlanDraft
	if a.def.Config.ApprovalPolicy == ApprovalAuto {
		planStatus = plan.PlanApproved
	} else if raw.Status == string(plan.PlanApproved) {
		planStatus = plan.PlanApproved
	}

	// Merge with existing plan — preserve status of known steps.
	existing, _ := a.planStore.Load()
	existingStatus := map[string]plan.Step{}
	if existing != nil {
		for _, s := range existing.Steps {
			existingStatus[s.ID] = s
		}
	}

	now := time.Now()
	steps := make([]plan.Step, 0, len(raw.Steps))
	for _, rs := range raw.Steps {
		step := plan.Step{
			ID:          rs.ID,
			Agent:       rs.Agent,
			Description: rs.Description,
			DependsOn:   rs.DependsOn,
			Status:      plan.StepPending,
		}
		if prev, ok := existingStatus[rs.ID]; ok {
			step.Status = prev.Status
			step.Result = prev.Result
			step.Error = prev.Error
			step.StartedAt = prev.StartedAt
			step.FinishedAt = prev.FinishedAt
		}
		steps = append(steps, step)
	}

	p := &plan.Plan{
		Goal:      raw.Goal,
		Status:    planStatus,
		Steps:     steps,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing != nil {
		p.CreatedAt = existing.CreatedAt
	}
	return a.planStore.Save(p)
}
