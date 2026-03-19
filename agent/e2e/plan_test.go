//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/quarkloop/agent/pkg/agent"
)

func TestPlanMode_CreatesPlan(t *testing.T) {
	a, k := newTestAgent(t, agent.ModePlan)

	resp := chat(t, a, "Write a Go function that checks if a number is prime and its unit test.", "plan")

	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}
	if resp.Mode != "plan" {
		t.Errorf("expected mode=plan, got: %s", resp.Mode)
	}

	// Verify a plan was stored in the KB.
	data, err := k.Get("plans", "master")
	if err != nil {
		t.Fatalf("expected plan in KB: %v", err)
	}

	var plan struct {
		Goal  string `json:"goal"`
		Steps []struct {
			ID string `json:"id"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Error("expected plan with at least one step")
	}
}

func TestPlanMode_ReturnsStructuredPlan(t *testing.T) {
	a, _ := newTestAgent(t, agent.ModePlan)

	resp := chat(t, a, "Create a REST API endpoint that returns user profiles.", "plan")

	// The response should contain JSON-like plan structure.
	if !strings.Contains(resp.Reply, "goal") || !strings.Contains(resp.Reply, "steps") {
		t.Errorf("expected plan JSON in reply, got: %s", truncate(resp.Reply, 200))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
