//go:build e2e

package e2e

import (
	"encoding/json"
	"testing"

	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/plan"
)

func TestMasterPlanMode_CreatesMasterPlan(t *testing.T) {
	a, k := newTestAgent(t, agent.ModeMasterPlan)

	resp := chat(t, a,
		"Build a full-stack e-commerce platform with product catalog, shopping cart, payment integration, and admin dashboard.",
		"masterplan",
	)

	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}
	if resp.Mode != "masterplan" {
		t.Errorf("expected mode=masterplan, got: %s", resp.Mode)
	}

	// Verify a master plan was stored in the KB.
	data, err := k.Get("plans", "masterplan")
	if err != nil {
		t.Fatalf("expected masterplan in KB: %v", err)
	}

	var mp plan.MasterPlan
	if err := json.Unmarshal(data, &mp); err != nil {
		t.Fatalf("unmarshal masterplan: %v", err)
	}
	if len(mp.Phases) == 0 {
		t.Error("expected masterplan with at least one phase")
	}
	if mp.Goal == "" {
		t.Error("expected masterplan to have a goal")
	}
}

func TestMasterPlanMode_PhasesHavePlanKeys(t *testing.T) {
	a, k := newTestAgent(t, agent.ModeMasterPlan)

	chat(t, a,
		"Design and implement a machine learning pipeline with data collection, preprocessing, model training, evaluation, and deployment.",
		"masterplan",
	)

	data, err := k.Get("plans", "masterplan")
	if err != nil {
		t.Fatalf("expected masterplan in KB: %v", err)
	}

	var mp plan.MasterPlan
	if err := json.Unmarshal(data, &mp); err != nil {
		t.Fatalf("unmarshal masterplan: %v", err)
	}

	for _, phase := range mp.Phases {
		if phase.PlanKey == "" {
			t.Errorf("phase %s has empty plan_key", phase.ID)
		}
		if phase.ID == "" {
			t.Error("phase has empty ID")
		}
	}
}
