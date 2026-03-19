//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/plan"
)

func TestPlanRun_UsesBashTool(t *testing.T) {
	restoreLogger := silenceStdLogger(t)
	defer restoreLogger()

	h := newLiveBashHarness(t, agent.ApprovalAuto)
	defer dumpRecentActivity(t, h.feed, 128)

	workDir := t.TempDir()
	targetFile := filepath.Join(workDir, "tool-bash-e2e.txt")
	marker := "tool-bash-e2e"

	planSingleStepBashTask(t, h.agent, []string{
		"pwd",
		fmt.Sprintf("printf '%s\\n' > '%s'", marker, targetFile),
		fmt.Sprintf("cat '%s'", targetFile),
	}, "Do not simulate output.")

	initialPlan := loadPlan(t, h.kb)
	if initialPlan.Status != plan.PlanApproved {
		t.Fatalf("expected approved plan before run, got: %s", mustJSON(t, initialPlan))
	}
	if len(initialPlan.Steps) == 0 {
		t.Fatalf("expected at least one step before run: %s", mustJSON(t, initialPlan))
	}
	if initialPlan.Steps[0].Agent != "supervisor" {
		t.Fatalf("expected supervisor step before run, got: %s", mustJSON(t, initialPlan))
	}
	if !strings.Contains(initialPlan.Steps[0].Description, targetFile) {
		t.Fatalf("expected plan step to include target path before run, got: %s", mustJSON(t, initialPlan))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- h.agent.Run(ctx)
	}()

	finalPlan := waitForPlanCompletion(t, h.kb, 60*time.Second)
	cancel()

	runErr := <-runDone
	if runErr != nil && runErr != context.Canceled && runErr != context.DeadlineExceeded {
		t.Fatalf("run failed: %v", runErr)
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("expected bash tool to create %s: %v", targetFile, err)
	}
	if got := strings.TrimSpace(string(content)); got != marker {
		t.Fatalf("expected file content %q, got %q", marker, got)
	}

	events := h.feed.Recent(128)
	bashCalls := 0
	bashResults := 0
	sawBashResultContent := false
	for _, ev := range events {
		data := eventData(ev)
		if data == nil || data["tool"] != "bash" {
			continue
		}
		switch ev.Type {
		case activity.ToolCalled:
			bashCalls++
		case activity.ToolCompleted:
			bashResults++
			if strings.Contains(data["output"], marker) || strings.Contains(data["result"], marker) {
				sawBashResultContent = true
			}
		}
	}
	if bashCalls < 2 {
		t.Fatalf("expected at least 2 bash tool calls, got %d", bashCalls)
	}
	if bashResults < 2 {
		t.Fatalf("expected at least 2 bash tool result events, got %d", bashResults)
	}
	if !sawBashResultContent {
		t.Fatalf("expected a bash tool.completed activity event to include output/result content %q", marker)
	}

	if !finalPlan.Complete {
		t.Fatalf("expected plan to complete, got: %s", mustJSON(t, finalPlan))
	}
	foundResult := false
	for _, step := range finalPlan.Steps {
		if step.Status != plan.StepComplete {
			t.Fatalf("expected completed plan steps, got: %s", mustJSON(t, finalPlan))
		}
		if strings.Contains(step.Result, marker) {
			foundResult = true
		}
	}
	if !foundResult {
		t.Fatalf("expected a completed step result to mention %q, got: %s", marker, mustJSON(t, finalPlan))
	}
}
