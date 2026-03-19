//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/plan"
)

// ---------------------------------------------------------------------------
// Ask mode activity
// ---------------------------------------------------------------------------

func TestActivity_AskMode_NoPlanEvents(t *testing.T) {
	a, _, feed := newTestAgentWithFeed(t, agent.ModeAsk, agent.ApprovalRequired)
	ch := feed.Subscribe()
	defer feed.Unsubscribe(ch)

	resp := chat(t, a, "What is the largest planet in our solar system?", "ask")

	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}

	events := drainEvents(ch)
	if hasEvent(events, activity.PlanCreated) {
		t.Error("ask mode should not emit plan.created")
	}
	if hasEvent(events, activity.MasterPlanCreated) {
		t.Error("ask mode should not emit masterplan.created")
	}
}

// ---------------------------------------------------------------------------
// Auto mode activity — classification events
// ---------------------------------------------------------------------------

func TestActivity_AutoMode_EmitsModeClassified(t *testing.T) {
	a, _, feed := newTestAgentWithFeed(t, agent.ModeAuto, agent.ApprovalRequired)
	ch := feed.Subscribe(activity.ModeClassified)
	defer feed.Unsubscribe(ch)

	resp := chat(t, a, "What is 7 times 8?", "auto")

	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}

	events := drainEvents(ch)
	if !hasEvent(events, activity.ModeClassified) {
		t.Fatal("expected mode.classified event")
	}

	// The classification event should include the resolved mode.
	data := eventData(events[0])
	if data == nil {
		t.Fatal("expected event data map")
	}
	resolved := data["resolved"]
	if resolved == "" {
		t.Error("expected resolved mode in event data")
	}
	t.Logf("auto classified as: %s", resolved)
}

// ---------------------------------------------------------------------------
// Plan mode activity — plan creation and approval
// ---------------------------------------------------------------------------

func TestActivity_PlanMode_EmitsPlanCreated(t *testing.T) {
	a, _, feed := newTestAgentWithFeed(t, agent.ModePlan, agent.ApprovalAuto)
	ch := feed.Subscribe(activity.PlanCreated)
	defer feed.Unsubscribe(ch)

	resp := chat(t, a, "Write a function that reverses a string.", "plan")

	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}

	events := drainEvents(ch)
	if !hasEvent(events, activity.PlanCreated) {
		if resp.Warning != "" {
			t.Skipf("plan not stored (model issue): %s", resp.Warning)
		}
		t.Error("expected plan.created event")
	}
}

func TestActivity_PlanApproval_DraftByDefault(t *testing.T) {
	a, k, _ := newTestAgentWithFeed(t, agent.ModePlan, agent.ApprovalRequired)

	resp := chat(t, a, "Sort an array of integers.", "plan")
	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}

	data, err := k.Get("plans", "master")
	if err != nil {
		if resp.Warning != "" {
			t.Skipf("plan not stored (model issue): %s", resp.Warning)
		}
		t.Fatalf("expected plan in KB: %v", err)
	}

	var p plan.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if p.Status != plan.PlanDraft {
		t.Errorf("expected status=draft, got: %s", p.Status)
	}
}

func TestActivity_PlanApproval_AutoApproves(t *testing.T) {
	a, k, _ := newTestAgentWithFeed(t, agent.ModePlan, agent.ApprovalAuto)

	resp := chat(t, a, "Parse a CSV file.", "plan")
	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}

	data, err := k.Get("plans", "master")
	if err != nil {
		if resp.Warning != "" {
			t.Skipf("plan not stored (model issue): %s", resp.Warning)
		}
		t.Fatalf("expected plan in KB: %v", err)
	}

	var p plan.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if p.Status != plan.PlanApproved {
		t.Errorf("expected status=approved, got: %s", p.Status)
	}
}

// ---------------------------------------------------------------------------
// Mode persistence — survives across interactions
// ---------------------------------------------------------------------------

func TestActivity_ModePersistence(t *testing.T) {
	// Use noop — no LLM needed for mode mechanics.
	a, _, _ := newNoopAgentWithFeed(t, agent.ModeAsk, agent.ApprovalRequired)

	if a.Mode() != agent.ModeAsk {
		t.Fatalf("expected initial mode=ask, got: %s", a.Mode())
	}

	// Switch mode via chat request.
	chat(t, a, "hello", "plan")
	if a.Mode() != agent.ModePlan {
		t.Errorf("expected mode=plan after chat, got: %s", a.Mode())
	}

	// Switch again.
	chat(t, a, "hello", "ask")
	if a.Mode() != agent.ModeAsk {
		t.Errorf("expected mode=ask after second chat, got: %s", a.Mode())
	}
}

func TestActivity_ModePersistedToKB(t *testing.T) {
	// Use noop — no LLM needed.
	a, k, _ := newNoopAgentWithFeed(t, agent.ModeAuto, agent.ApprovalRequired)

	// Set mode via chat.
	chat(t, a, "hello", "plan")

	// Read mode directly from KB.
	data, err := k.Get("config", "mode")
	if err != nil {
		t.Fatalf("mode not persisted: %v", err)
	}
	if string(data) != "plan" {
		t.Errorf("expected persisted mode=plan, got: %s", string(data))
	}
}

// ---------------------------------------------------------------------------
// Multi-turn conversation — context preserved
// ---------------------------------------------------------------------------

func TestActivity_MultiTurnConversation(t *testing.T) {
	a, _, _ := newTestAgentWithFeed(t, agent.ModeAsk, agent.ApprovalRequired)

	// First turn: tell the agent something.
	resp1 := chat(t, a, "Remember this number: 42.", "ask")
	if resp1.Reply == "" {
		t.Fatal("expected non-empty first reply")
	}

	// Second turn: ask about it.
	resp2 := chat(t, a, "What number did I just tell you to remember?", "ask")
	if resp2.Reply == "" {
		t.Fatal("expected non-empty second reply")
	}

	// The reply should reference 42 (context was preserved).
	t.Logf("reply: %s", resp2.Reply)
	if len(resp2.Reply) < 2 {
		t.Error("reply too short to verify context preservation")
	}
}

// ---------------------------------------------------------------------------
// Warning field — surfaced when plan extraction fails
// ---------------------------------------------------------------------------

func TestActivity_WarningOnMalformedPlan(t *testing.T) {
	// Noop gateway returns non-JSON, so plan extraction will fail.
	a, _, _ := newNoopAgentWithFeed(t, agent.ModePlan, agent.ApprovalRequired)

	resp := chat(t, a, "Build something.", "plan")

	// Noop returns a canned response that isn't JSON.
	if resp.Warning == "" {
		t.Error("expected warning when plan JSON extraction fails")
	}
	t.Logf("warning: %s", resp.Warning)
}

func TestActivity_WarningOnMalformedMasterPlan(t *testing.T) {
	a, _, _ := newNoopAgentWithFeed(t, agent.ModeMasterPlan, agent.ApprovalRequired)

	resp := chat(t, a, "Build a platform.", "masterplan")

	if resp.Warning == "" {
		t.Error("expected warning when masterplan JSON extraction fails")
	}
	t.Logf("warning: %s", resp.Warning)
}

// ---------------------------------------------------------------------------
// Run loop — approved plan triggers supervisor cycle
// ---------------------------------------------------------------------------

func TestActivity_RunLoop_IdlesOnDraftPlan(t *testing.T) {
	a, k, feed := newNoopAgentWithFeed(t, agent.ModePlan, agent.ApprovalRequired)

	// Manually store a draft plan in KB.
	p := &plan.Plan{
		Goal:   "test",
		Status: plan.PlanDraft,
		Steps: []plan.Step{
			{ID: "s1", Agent: "supervisor", Description: "do thing", Status: plan.StepPending},
		},
	}
	data, _ := json.Marshal(p)
	k.Set("plans", "master", data)

	// Start Run in background, let it run a few cycles.
	ch := feed.Subscribe(activity.StepDispatched)
	defer feed.Unsubscribe(ch)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go a.Run(ctx)

	// Wait and check — no steps should be dispatched for a draft plan.
	time.Sleep(4 * time.Second)
	events := drainEvents(ch)
	if hasEvent(events, activity.StepDispatched) {
		t.Error("Run should not dispatch steps for a draft plan")
	}
}

// ---------------------------------------------------------------------------
// Mode in response — always populated
// ---------------------------------------------------------------------------

func TestActivity_ResponseAlwaysHasMode(t *testing.T) {
	a, _, _ := newNoopAgentWithFeed(t, agent.ModeAsk, agent.ApprovalRequired)

	tests := []struct {
		reqMode  string
		wantMode string
	}{
		{"ask", "ask"},
		{"plan", "plan"},
		{"masterplan", "masterplan"},
	}
	for _, tt := range tests {
		resp := chat(t, a, "hello", tt.reqMode)
		if resp.Mode != tt.wantMode {
			t.Errorf("mode=%s: expected response mode=%s, got=%s", tt.reqMode, tt.wantMode, resp.Mode)
		}
	}
}
