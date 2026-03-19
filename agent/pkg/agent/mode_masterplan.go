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

// chatMasterPlan handles a chat request in masterplan mode. It calls the LLM
// to produce a master plan with phases, stores it in the KB, and returns it
// to the user. The supervisor LLM decides how to execute — this handler does
// not drive execution.
func (a *Agent) chatMasterPlan(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Inject existing masterplan context.
	message := a.withMasterPlanContext(req.Message)

	var cr *ChatResponse

	// Streaming path.
	if req.Stream {
		if sg, ok := a.gateway.(model.StreamingGateway); ok {
			resp, err := a.chatStream(ctx, sg, message)
			if err != nil {
				return nil, fmt.Errorf("masterplan stream: %w", err)
			}
			cr = resp
			cr.Mode = string(ModeMasterPlan)
		}
	}

	// Non-streaming path.
	if cr == nil {
		resp, err := a.inferWithContext(ctx, a.ctx, message)
		if err != nil {
			return nil, fmt.Errorf("masterplan infer: %w", err)
		}
		cr = &ChatResponse{
			Reply:        resp.Content,
			Mode:         string(ModeMasterPlan),
			InputTokens:  resp.InputTokens,
			OutputTokens: resp.OutputTokens,
		}
	}

	// Empty response check.
	if cr.Reply == "" {
		cr.Warning = "model returned an empty response — masterplan was not created"
		a.saveCheckpoint()
		return cr, nil
	}

	// Try to extract and store the masterplan.
	mpData, err := extractJSON(cr.Reply)
	if err != nil {
		cr.Warning = fmt.Sprintf("masterplan was not stored: %v", err)
		log.Printf("agent: masterplan JSON extraction failed: %v", err)
	} else if err := a.storeMasterPlanFromResponse(mpData); err != nil {
		cr.Warning = fmt.Sprintf("masterplan was not stored: %v", err)
		log.Printf("agent: failed to store masterplan: %v", err)
	} else {
		a.emit(activity.MasterPlanCreated, map[string]string{"mode": "masterplan"})
	}

	a.saveCheckpoint()
	return cr, nil
}

// withMasterPlanContext prepends existing masterplan state to the user message.
func (a *Agent) withMasterPlanContext(message string) string {
	existing, err := a.masterPlanStore.Load()
	if err != nil || existing == nil || existing.Complete {
		return message
	}
	mpJSON, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return message
	}
	return fmt.Sprintf("Current masterplan:\n```json\n%s\n```\n\n%s", string(mpJSON), message)
}

// storeMasterPlanFromResponse parses a JSON master plan and persists it to KB.
func (a *Agent) storeMasterPlanFromResponse(data []byte) error {
	var raw struct {
		Goal   string `json:"goal"`
		Vision string `json:"vision"`
		Status string `json:"status"` // "draft" or "approved"
		Phases []struct {
			ID          string   `json:"id"`
			Description string   `json:"description"`
			DependsOn   []string `json:"depends_on"`
		} `json:"phases"`
		Complete bool   `json:"complete"`
		Summary  string `json:"summary"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing masterplan: %w", err)
	}

	if raw.Complete {
		existing, _ := a.masterPlanStore.Load()
		if existing == nil {
			existing = &plan.MasterPlan{}
		}
		existing.Complete = true
		existing.Summary = raw.Summary
		existing.UpdatedAt = time.Now()
		return a.masterPlanStore.Save(existing)
	}

	if len(raw.Phases) == 0 {
		return fmt.Errorf("masterplan has no phases")
	}

	// Resolve plan status: auto-approve if approval policy says so,
	// otherwise respect whatever the LLM returned.
	planStatus := plan.PlanDraft
	if a.def.Config.ApprovalPolicy == ApprovalAuto {
		planStatus = plan.PlanApproved
	} else if raw.Status == string(plan.PlanApproved) {
		planStatus = plan.PlanApproved
	}

	now := time.Now()
	phases := make([]plan.Phase, 0, len(raw.Phases))
	for _, rp := range raw.Phases {
		phases = append(phases, plan.Phase{
			ID:          rp.ID,
			Description: rp.Description,
			PlanKey:     fmt.Sprintf("phase-%s", rp.ID),
			DependsOn:   rp.DependsOn,
			Status:      plan.StepPending,
		})
	}

	mp := &plan.MasterPlan{
		Goal:      raw.Goal,
		Vision:    raw.Vision,
		Status:    planStatus,
		Phases:    phases,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return a.masterPlanStore.Save(mp)
}
