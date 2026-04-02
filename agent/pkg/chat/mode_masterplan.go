package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/inference"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/plan"
)

// processMasterPlan handles a chat request in masterplan mode.
func processMasterPlan(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	deps *Deps,
	req agentcore.ChatRequest,
) (*agentcore.ChatResponse, error) {
	message := withMasterPlanContext(deps.MasterStore, req.Message)

	var cr *agentcore.ChatResponse

	if req.Stream {
		if sg, ok := res.GetGateway().(model.StreamingGateway); ok {
			resp, err := chatStream(ctx, ac, res, deps, sg, message)
			if err != nil {
				return nil, fmt.Errorf("masterplan stream: %w", err)
			}
			cr = resp
			cr.Mode = string(agentcore.ModeMasterPlan)
		}
	}

	if cr == nil {
		resp, err := inference.Infer(ctx, ac, res, message)
		if err != nil {
			return nil, fmt.Errorf("masterplan infer: %w", err)
		}
		cr = &agentcore.ChatResponse{
			Reply:        resp.Content,
			Mode:         string(agentcore.ModeMasterPlan),
			InputTokens:  resp.InputTokens,
			OutputTokens: resp.OutputTokens,
		}
	}

	if cr.Reply == "" {
		cr.Warning = "model returned an empty response — masterplan was not created"
		return cr, nil
	}

	mpData, err := extractJSON(cr.Reply)
	if err != nil {
		cr.Warning = fmt.Sprintf("masterplan was not stored: %v", err)
		log.Printf("chat: masterplan JSON extraction failed: %v", err)
	} else if err := storeMasterPlanFromResponse(deps, mpData); err != nil {
		cr.Warning = fmt.Sprintf("masterplan was not stored: %v", err)
		log.Printf("chat: failed to store masterplan: %v", err)
	} else {
		emitActivity(res.EventBus, req.SessionKey, eventbus.KindMasterPlanCreated, map[string]string{"mode": "masterplan"})
	}

	return cr, nil
}

func withMasterPlanContext(masterStore *plan.MasterPlanStore, message string) string {
	existing, err := masterStore.Load()
	if err != nil || existing == nil || existing.Complete {
		return message
	}
	mpJSON, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return message
	}
	return fmt.Sprintf("Current masterplan:\n```json\n%s\n```\n\n%s", string(mpJSON), message)
}

func storeMasterPlanFromResponse(deps *Deps, data []byte) error {
	var raw struct {
		Goal   string `json:"goal"`
		Vision string `json:"vision"`
		Status string `json:"status"`
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
		existing, _ := deps.MasterStore.Load()
		if existing == nil {
			existing = &plan.MasterPlan{}
		}
		existing.Complete = true
		existing.Summary = raw.Summary
		existing.UpdatedAt = time.Now()
		return deps.MasterStore.Save(existing)
	}

	if len(raw.Phases) == 0 {
		return fmt.Errorf("masterplan has no phases")
	}

	planStatus := plan.PlanDraft
	if deps.Def.Config.ApprovalPolicy == agentcore.ApprovalAuto {
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
	return deps.MasterStore.Save(mp)
}
