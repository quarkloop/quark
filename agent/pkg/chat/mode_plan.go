package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/execution"
	"github.com/quarkloop/agent/pkg/inference"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/plan"
)

// processPlan handles a chat request in plan mode.
func processPlan(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	deps *Deps,
	req agentcore.ChatRequest,
) (*agentcore.ChatResponse, error) {
	message := withPlanContext(deps.PlanStore, req.Message)

	var cr *agentcore.ChatResponse

	// Streaming path.
	if req.Stream {
		if sg, ok := res.Gateway.(model.StreamingGateway); ok {
			resp, err := chatStream(ctx, ac, res, deps, sg, message)
			if err != nil {
				return nil, fmt.Errorf("plan stream: %w", err)
			}
			cr = resp
			cr.Mode = string(agentcore.ModePlan)
		}
	}

	// Non-streaming path.
	if cr == nil {
		resp, err := inference.Infer(ctx, ac, res, message)
		if err != nil {
			return nil, fmt.Errorf("plan infer: %w", err)
		}
		cr = &agentcore.ChatResponse{
			Reply:        resp.Content,
			Mode:         string(agentcore.ModePlan),
			InputTokens:  resp.InputTokens,
			OutputTokens: resp.OutputTokens,
		}
	}

	if cr.Reply == "" {
		cr.Warning = "model returned an empty response — plan was not created"
		return cr, nil
	}

	planData, err := extractJSON(cr.Reply)
	if err != nil {
		cr.Warning = fmt.Sprintf("plan was not stored: %v", err)
		log.Printf("chat: plan JSON extraction failed: %v reply=%q", err, execution.Truncate(cr.Reply, 512))
	} else if err := storePlanFromResponse(deps, planData); err != nil {
		cr.Warning = fmt.Sprintf("plan was not stored: %v", err)
		log.Printf("chat: failed to store plan: %v raw=%q", err, execution.Truncate(string(planData), 512))
	} else {
		emitActivity(res.Activity, req.SessionKey, activity.PlanCreated, map[string]string{"mode": "plan"})
	}

	return cr, nil
}

func withPlanContext(planStore *plan.Store, message string) string {
	existing, err := planStore.Load()
	if err != nil || existing == nil || existing.Complete {
		return message
	}
	planJSON, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return message
	}
	return fmt.Sprintf("Current plan:\n```json\n%s\n```\n\n%s", string(planJSON), message)
}

func storePlanFromResponse(deps *Deps, data []byte) error {
	var raw struct {
		Goal   string `json:"goal"`
		Status string `json:"status"`
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

	if raw.Complete {
		existing, _ := deps.PlanStore.Load()
		if existing == nil {
			existing = &plan.Plan{}
		}
		existing.Complete = true
		existing.Summary = raw.Summary
		existing.UpdatedAt = time.Now()
		return deps.PlanStore.Save(existing)
	}

	if len(raw.Steps) == 0 {
		return fmt.Errorf("plan has no steps")
	}

	planStatus := plan.PlanDraft
	if deps.Def.Config.ApprovalPolicy == agentcore.ApprovalAuto {
		planStatus = plan.PlanApproved
	} else if raw.Status == string(plan.PlanApproved) {
		planStatus = plan.PlanApproved
	}

	existing, _ := deps.PlanStore.Load()
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
	return deps.PlanStore.Save(p)
}

// chatStream performs a streaming chat call, collecting the full response.
func chatStream(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	deps *Deps,
	sg model.StreamingGateway,
	message string,
) (*agentcore.ChatResponse, error) {
	adapter, err := res.AdapterReg.Get(res.Gateway.Provider())
	if err != nil {
		return nil, fmt.Errorf("chat stream: adapter: %w", err)
	}
	ca := llmctx.NewContextAdapter(ac, adapter)
	payload, err := ca.BuildRequest(llmctx.RequestOptions{
		Model:     res.Gateway.ModelName(),
		MaxTokens: res.Gateway.MaxTokens(),
	})
	if err != nil {
		return nil, fmt.Errorf("chat stream: build request: %w", err)
	}

	ch, err := sg.InferStream(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("chat stream: infer: %w", err)
	}

	content, err := model.CollectStream(ch)
	if err != nil {
		return nil, fmt.Errorf("chat stream: collect: %w", err)
	}

	if message != "" {
		if m, err := inference.NewUserMessage(res.TC, res.IDGen, agentcore.AuthorUser, message); err == nil {
			ac.AppendMessage(ctx, m)
		}
	}
	if agtMsg, err := inference.NewAgentMessage(res.TC, res.IDGen, agentcore.AuthorAgent, content); err == nil {
		ac.AppendMessage(ctx, agtMsg)
	}

	return &agentcore.ChatResponse{Reply: content}, nil
}
