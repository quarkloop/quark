//go:build e2e

package e2e

import (
	"testing"

	"github.com/quarkloop/agent/pkg/agent"
)

func TestAutoMode_ClassifiesQuestion(t *testing.T) {
	a, _ := newTestAgent(t, agent.ModeAuto)

	resp := chat(t, a, "What is the capital of France?", "auto")

	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}
	// After classification, the mode should have resolved to "ask".
	if resp.Mode != "ask" {
		t.Logf("auto classified as %s (expected ask) — may vary by LLM", resp.Mode)
	}
}

func TestAutoMode_ClassifiesTask(t *testing.T) {
	a, _ := newTestAgent(t, agent.ModeAuto)

	resp := chat(t, a, "Write a Python script that scrapes weather data from an API and saves it to a CSV file.", "auto")

	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}
	// After classification, the mode should have resolved to "plan".
	if resp.Mode != "plan" && resp.Mode != "masterplan" {
		t.Logf("auto classified as %s (expected plan or masterplan) — may vary by LLM", resp.Mode)
	}
}

func TestAutoMode_DefaultsToAuto(t *testing.T) {
	a, _ := newTestAgent(t, agent.ModeAuto)

	// Send without explicit mode — should use agent's default (auto).
	resp := chat(t, a, "Hello, how are you?", "")

	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}
	// The response mode should reflect whatever auto resolved to.
	if resp.Mode == "" {
		t.Error("expected mode to be set in response")
	}
}
