//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/quarkloop/agent/pkg/agent"
)

func TestAskMode_AnswersQuestion(t *testing.T) {
	a, _ := newTestAgent(t, agent.ModeAsk)

	resp := chat(t, a, "What is 2+2? Reply with just the number.", "ask")

	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}
	if !strings.Contains(resp.Reply, "4") {
		t.Errorf("expected reply to contain '4', got: %s", resp.Reply)
	}
	if resp.Mode != "ask" {
		t.Errorf("expected mode=ask, got: %s", resp.Mode)
	}
}

func TestAskMode_DoesNotCreatePlan(t *testing.T) {
	a, k := newTestAgent(t, agent.ModeAsk)

	chat(t, a, "Create a detailed plan to build a website with user authentication.", "ask")

	// Verify no plan was created in KB.
	if _, err := k.Get("plans", "master"); err == nil {
		t.Error("expected no plan in KB, but found one")
	}
}
