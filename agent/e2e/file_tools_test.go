//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/core/pkg/kb"
)

func TestPlanRun_UsesReadAndWriteToolsForPythonCreateAndEdit(t *testing.T) {
	h := newLiveFileToolsHarness(t, agent.ApprovalAuto)
	defer dumpRecentActivity(t, h.feed, 256)

	workDir := t.TempDir()
	targetFile := filepath.Join(workDir, "greet.py")
	planFileToolGoal(t, h.agent, h.kb, "write", []string{"write"}, []string{
		"Create a focused one-step supervisor plan for a Python coding task.",
		"Create a new Python file at " + targetFile + ".",
		"The file must be valid runnable multi-line Python code.",
		"Add a small greeting function or method that takes a name and produces the greeting Hello, <name>!.",
		"When the file is executed with python3, it must print exactly Hello, World!.",
		"Use the write tool to create the real file and keep the work in one supervisor step.",
	})

	firstPlan := runPlanToCompletion(t, h.agent, h.kb, 120*time.Second, 120*time.Second)
	assertCompletedPlanHasResult(t, firstPlan)

	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("expected write tool to create %s: %v", targetFile, err)
	}
	initialCode := string(content)
	if !strings.Contains(initialCode, "def ") {
		t.Fatalf("expected created file to define a Python function, got %q", initialCode)
	}
	if !strings.Contains(initialCode, "World") {
		t.Fatalf("expected created file to reference World, got %q", initialCode)
	}
	requirePythonOutput(t, targetFile, "Hello, World!")

	planFileToolGoal(t, h.agent, h.kb, "read", []string{"read", "write"}, []string{
		"Create a focused one-step supervisor plan to update the existing Python file at " + targetFile + ".",
		"Inspect the current file with the read tool before editing it.",
		"Update the code in place rather than rewriting the whole file from scratch.",
		"Use the write tool edit operation for this update.",
		"Figure out the needed edit ranges from the current file content after the read result.",
		"Update the greeting function or method so it accepts an optional punctuation argument that defaults to !.",
		"When the file is executed with python3 after the update, it must print exactly Hello, Quark?.",
		"Do the real edit on the existing file and keep the work in one supervisor step.",
	})

	secondPlan := runPlanToCompletion(t, h.agent, h.kb, 240*time.Second, 240*time.Second)
	assertCompletedPlanHasResult(t, secondPlan)

	content, err = os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("expected edited python file at %s: %v", targetFile, err)
	}
	updatedCode := string(content)
	if !strings.Contains(updatedCode, "punctuation") {
		t.Fatalf("expected updated file to use punctuation parameter, got %q", updatedCode)
	}
	if !strings.Contains(updatedCode, "Quark") {
		t.Fatalf("expected updated file to reference Quark, got %q", updatedCode)
	}
	requirePythonOutput(t, targetFile, "Hello, Quark?")

	events := h.feed.Recent(256)
	sawEditCall := false
	sawEditResult := false
	sawUpdatedPreview := false
	sawReadCall := false
	sawReadResult := false
	sawReadContent := false
	for _, ev := range events {
		data := eventData(ev)
		if data == nil {
			continue
		}
		switch ev.Type {
		case activity.ToolCalled:
			if data["tool"] == "write" && data["operation"] == "edit" {
				sawEditCall = true
			}
			if data["tool"] == "read" && data["path"] == targetFile {
				sawReadCall = true
			}
		case activity.ToolCompleted:
			if data["tool"] == "read" && data["path"] == targetFile {
				sawReadResult = true
				if strings.Contains(data["content"], "def greet") ||
					strings.Contains(data["result"], "def greet") {
					sawReadContent = true
				}
			}
			if data["tool"] == "write" && data["operation"] == "edit" {
				sawEditResult = true
				if strings.Contains(data["content_preview"], "punctuation") ||
					strings.Contains(data["result"], "punctuation") {
					sawUpdatedPreview = true
				}
			}
		}
	}
	if !sawReadCall {
		t.Fatal("expected at least one read inspection call while preparing the code edit")
	}
	if !sawReadResult {
		t.Fatal("expected a read tool.completed activity event for the target file")
	}
	if !sawReadContent {
		t.Fatal("expected a read tool.completed activity event to include inspected Python content")
	}
	if !sawEditCall {
		t.Fatal("expected a write tool.called activity event for operation=edit")
	}
	if !sawEditResult {
		t.Fatal("expected a write tool.completed activity event for operation=edit")
	}
	if !sawUpdatedPreview {
		t.Fatal("expected a write tool.completed activity event to include updated Python preview content")
	}
}

func planFileToolGoal(t *testing.T, a *agent.Agent, k kb.Store, firstTool string, requiredTools []string, lines []string) {
	t.Helper()

	goal := strings.Join(lines, "\n")
	resp := requireStoredPlan(t, a, []string{
		goal,
		goal + "\nReturn a stored execution plan for this exact goal.",
		goal + "\nRetry and store the plan. Use the required file tools to complete the task for real.",
	})
	if resp.Mode != "plan" {
		t.Fatalf("expected mode=plan, got: %s", resp.Mode)
	}

	for i := 0; i < 3; i++ {
		p := loadPlan(t, k)
		if len(p.Steps) == 1 && p.Steps[0].Agent == "supervisor" &&
			stepDescriptionSatisfiesFileToolRequirements(p.Steps[0].Description, firstTool, requiredTools...) {
			break
		}

		requireStoredPlan(t, a, []string{
			"Revise the current plan so it has exactly one step assigned to supervisor.\n" +
				"The read and write entries are tools, not agents, so do not use them in the agent field.\n" +
				"Keep the goal the same and keep execution in one supervisor step.\n" +
				fileToolStepRequirements(firstTool),
			"Update the current plan.\n" +
				"Use exactly one step.\n" +
				"Set the step agent to supervisor.\n" +
				"Use the file tools inside that step; do not set agent to a tool name.\n" +
				fileToolStepRequirements(firstTool),
			"Retry the current plan revision.\n" +
				"The plan must have one supervisor step only.\n" +
				"read and write are only tools and must not appear as agent names.\n" +
				fileToolStepRequirements(firstTool),
		})
	}

	p := loadPlan(t, k)
	if len(p.Steps) != 1 || p.Steps[0].Agent != "supervisor" ||
		!stepDescriptionSatisfiesFileToolRequirements(p.Steps[0].Description, firstTool, requiredTools...) {
		t.Fatalf("expected a one-step supervisor plan before run, got: %s", mustJSON(t, p))
	}

	requireStoredPlan(t, a, []string{
		"Revise the current one-step supervisor plan so the step description explicitly says:\n" +
			fileToolPlanChecklist(firstTool) +
			"Keep the goal the same.",
		"Update the current supervisor step description.\n" +
			fileToolStepRequirements(firstTool) + "\n" +
			"Do not change the goal.",
		"Retry the current plan revision.\n" +
			fileToolStepRequirements(firstTool),
	})

	p = loadPlan(t, k)
	if len(p.Steps) != 1 || p.Steps[0].Agent != "supervisor" ||
		!stepDescriptionSatisfiesFileToolRequirements(p.Steps[0].Description, firstTool, requiredTools...) {
		t.Fatalf("expected refined one-step supervisor plan before run, got: %s", mustJSON(t, p))
	}
}

func stepDescriptionSatisfiesFileToolRequirements(description, firstTool string, tools ...string) bool {
	desc := strings.ToLower(description)
	for _, tool := range tools {
		if !strings.Contains(desc, strings.ToLower(tool)+" skill") {
			return false
		}
	}

	requiredPhrases := []string{
		fmt.Sprintf("execution response must be a %s tool call", firstTool),
		fmt.Sprintf("plain text before a %s tool call is invalid", firstTool),
	}
	if firstTool == "read" {
		requiredPhrases = append(requiredPhrases,
			"read tool before editing",
			"write tool",
			"before any summary",
		)
	} else {
		requiredPhrases = append(requiredPhrases,
			"write tool",
			"before any summary",
		)
	}

	for _, phrase := range requiredPhrases {
		if !strings.Contains(desc, phrase) {
			return false
		}
	}
	return true
}

func fileToolStepRequirements(firstTool string) string {
	if firstTool == "read" {
		return "The step description must explicitly say the first execution response must be a read tool call, that plain text before a read tool call is invalid, that the worker must inspect the current file with the read tool before editing it, and that after inspection it must use the write tool to make the real on-disk changes before any summary."
	}
	return "The step description must explicitly say the first execution response must be a write tool call, that plain text before a write tool call is invalid, and that it must use the write tool to make the real on-disk changes before any summary."
}

func fileToolPlanChecklist(firstTool string) string {
	if firstTool == "read" {
		return "- the first execution response must be a read tool call\n" +
			"- plain text before a read tool call is invalid\n" +
			"- the worker must inspect the current file with the read tool before editing it\n" +
			"- after inspection the worker must use the write tool to make the real on-disk changes before any summary\n"
	}
	return "- the first execution response must be a write tool call\n" +
		"- plain text before a write tool call is invalid\n" +
		"- it must use the write tool to make the real on-disk changes before any summary\n"
}

func assertCompletedPlanHasResult(t *testing.T, p *plan.Plan) {
	t.Helper()

	if !p.Complete {
		t.Fatalf("expected plan to complete, got: %s", mustJSON(t, p))
	}

	foundNonEmptyResult := false
	for _, step := range p.Steps {
		if step.Status != plan.StepComplete {
			t.Fatalf("expected completed plan steps, got: %s", mustJSON(t, p))
		}
		if strings.TrimSpace(step.Result) != "" {
			foundNonEmptyResult = true
		}
	}
	if !foundNonEmptyResult {
		t.Fatalf("expected a completed step result, got: %s", mustJSON(t, p))
	}
}

func requirePythonOutput(t *testing.T, path, want string) {
	t.Helper()

	out, err := exec.Command("python3", path).CombinedOutput()
	if err != nil {
		t.Fatalf("python3 %s failed: %v\n%s", path, err, string(out))
	}
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("expected python output %q, got %q", want, got)
	}
}
