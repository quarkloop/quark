//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"
)

func TestAgentBinary_FileTools_CreateAndEditPython(t *testing.T) {
	cfg := resolveProviderConfig(t)

	homeDir := t.TempDir()
	scaffoldRegistryForTestHome(t, homeDir)

	binaries := buildRequiredBinaries(t)

	bashPort := reservePort(t)
	readPort := reservePort(t)
	writePort := reservePort(t)
	agentPort := reservePort(t)

	bashURL := fmt.Sprintf("http://127.0.0.1:%d/run", bashPort)
	readURL := fmt.Sprintf("http://127.0.0.1:%d/read", readPort)
	writeURL := fmt.Sprintf("http://127.0.0.1:%d/apply", writePort)
	projectDir := createFileToolsProject(t, cfg, bashURL, readURL, writeURL)

	commonEnv := processEnv(map[string]string{
		"HOME": homeDir,
	})

	startProcess(t, "bash-tool", binaries.bash, []string{"serve", "--addr", fmt.Sprintf("127.0.0.1:%d", bashPort)}, commonEnv)
	startProcess(t, "read-tool", binaries.read, []string{"serve", "--addr", fmt.Sprintf("127.0.0.1:%d", readPort)}, commonEnv)
	startProcess(t, "write-tool", binaries.write, []string{"serve", "--addr", fmt.Sprintf("127.0.0.1:%d", writePort)}, commonEnv)
	waitForPort(t, bashPort, 10*time.Second)
	waitForPort(t, readPort, 10*time.Second)
	waitForPort(t, writePort, 10*time.Second)

	agentEnv := processEnv(map[string]string{
		"HOME":                  homeDir,
		"QUARK_APPROVAL_POLICY": string("auto"),
		"OPENROUTER_API_KEY":    os.Getenv("OPENROUTER_API_KEY"),
		"ZHIPU_API_KEY":         os.Getenv("ZHIPU_API_KEY"),
		"OPENAI_API_KEY":        os.Getenv("OPENAI_API_KEY"),
		"ANTHROPIC_API_KEY":     os.Getenv("ANTHROPIC_API_KEY"),
	})
	startProcess(t, "agent-runtime", binaries.agent, []string{
		"run",
		"--id", "e2e-file-tools",
		"--dir", projectDir,
		"--port", fmt.Sprintf("%d", agentPort),
	}, agentEnv)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", agentPort)
	client := agentclient.New(
		baseURL,
		agentclient.WithTransport(agentclient.NewTransport(baseURL, agentclient.WithTimeout(5*time.Minute))),
	)
	waitForAgentReady(t, client, 30*time.Second)

	targetFile := filepath.Join(projectDir, "greet.py")

	createPrompts := []string{strings.Join([]string{
		"Create a focused one-step supervisor plan for a Python coding task.",
		"Create a new Python file at " + targetFile + ".",
		"Use the exact absolute file path " + targetFile + " in the write tool call. Do not switch to a relative path.",
		"The file must be valid runnable multi-line Python code.",
		"Add a small greeting function or method.",
		"When the file is executed with python3, it must print exactly Hello, World!.",
		"Use the write tool to create the real file and keep the work in one supervisor step.",
		"Use the bash tool to run python3 on the created file and verify the exact output before any summary.",
		"The step description must explicitly say the first execution response must be a write tool call, that plain text before a write tool call is invalid, that the write tool must make the real on-disk changes before any summary, and that the bash tool must verify the program output before any summary.",
		"Respond with only valid JSON that matches the plan schema and includes exactly one step in the steps array.",
	}, "\n"),
		strings.Join([]string{
			"Retry the same Python file creation request.",
			"Return a valid JSON plan with exactly one step assigned to supervisor.",
			"Create the file at " + targetFile + " with the write tool.",
			"Use the exact absolute file path " + targetFile + " in the write tool call.",
			"When the file is executed with python3, it must print exactly Hello, World!.",
			"The single step description must say the first execution response must be a write tool call, that the write tool must make the real on-disk changes before any summary, and that the bash tool must verify the program output before any summary.",
			"Respond with only JSON. The steps array must contain exactly one step.",
		}, "\n"),
		strings.Join([]string{
			"Create a fresh valid JSON execution plan with exactly one supervisor step.",
			"The task is to create " + targetFile + " as runnable Python code.",
			"Use the write tool to make the real file.",
			"Use the exact absolute file path " + targetFile + " in the write tool call.",
			"When the file is executed with python3, it must print exactly Hello, World!.",
			"Use the bash tool to verify the exact python3 output before any summary.",
			"Do not return a completion-only object. Return goal, status, and a steps array with exactly one step.",
		}, "\n"),
	}

	createStartedAt := time.Now().Add(-time.Second)
	createResp := requireStoredPlanViaClient(t, client, createPrompts)
	if createResp.Mode != "plan" {
		t.Fatalf("expected mode=plan for create request, got %q", createResp.Mode)
	}

	firstPlan := waitForCompletedPlanAfter(t, client, createStartedAt, 4*time.Minute)
	if !firstPlan.Complete {
		t.Fatalf("expected first plan to complete, got: %s", mustJSON(t, firstPlan))
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("expected created file %s: %v", targetFile, err)
	}
	initialCode := string(content)
	if strings.TrimSpace(initialCode) == "" {
		t.Fatal("expected created file to be non-empty")
	}
	requirePythonOutput(t, targetFile, "Hello, World!")

	updatePrompts := []string{strings.Join([]string{
		"Create a focused one-step supervisor plan to update the existing Python file at " + targetFile + ".",
		"Inspect the current file with the read tool before editing it.",
		"Use the exact absolute file path " + targetFile + " in both read and write tool calls.",
		"Update the code in place rather than rewriting the whole file from scratch.",
		"Use the write tool edit operation for this update.",
		"Update the greeting code so the program prints Hello, Quark! when executed.",
		"When the file is executed with python3 after the update, it must print exactly Hello, Quark!.",
		"Do the real edit on the existing file and keep the work in one supervisor step.",
		"Use the bash tool to run python3 on the updated file and verify the exact output before any summary.",
		"The step description must explicitly say the first execution response must be a read tool call, that plain text before a read tool call is invalid, that the read tool must inspect the current file before editing it, that the write tool must make the real on-disk changes before any summary, and that the bash tool must verify the program output before any summary.",
		"Respond with only valid JSON that matches the plan schema and includes exactly one step in the steps array.",
	}, "\n"),
		strings.Join([]string{
			"Retry the same Python update request with a valid JSON plan.",
			"Use exactly one supervisor step.",
			"Inspect the current file with the read tool before editing it.",
			"Use the exact absolute file path " + targetFile + " in both read and write tool calls.",
			"Use the write tool edit operation to update the existing file so python3 prints exactly Hello, Quark!.",
			"The single step description must say the first execution response must be a read tool call, that the write tool must make the real on-disk changes before any summary, and that the bash tool must verify the program output before any summary.",
			"Respond with only JSON. The steps array must contain exactly one step.",
		}, "\n"),
		strings.Join([]string{
			"Create a fresh valid JSON execution plan with exactly one supervisor step.",
			"Inspect " + targetFile + " with read before editing it.",
			"Use the exact absolute file path " + targetFile + " in both read and write tool calls.",
			"Use write edit to update the existing file so python3 prints exactly Hello, Quark!.",
			"Use the bash tool to verify the exact python3 output before any summary.",
			"The step description must say the read tool must inspect the file before editing it, the write tool must make the real on-disk changes before any summary, and the bash tool must verify the program output before any summary.",
			"Do not return a completion-only object. Return goal, status, and a steps array with exactly one step.",
		}, "\n"),
	}

	updateStartedAt := time.Now().Add(-time.Second)
	updateResp := requireStoredPlanViaClient(t, client, updatePrompts)
	if updateResp.Mode != "plan" {
		t.Fatalf("expected mode=plan for update request, got %q", updateResp.Mode)
	}

	secondPlan := waitForCompletedPlanAfter(t, client, updateStartedAt, 6*time.Minute)
	if !secondPlan.Complete {
		t.Fatalf("expected second plan to complete, got: %s", mustJSON(t, secondPlan))
	}

	content, err = os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("expected edited file %s: %v", targetFile, err)
	}
	updatedCode := string(content)
	if !strings.Contains(updatedCode, "Quark") {
		t.Fatalf("expected updated file to reference Quark, got %q", updatedCode)
	}
	requirePythonOutput(t, targetFile, "Hello, Quark!")

	events, err := client.Activity(context.Background(), 256)
	if err != nil {
		t.Fatalf("load agent activity: %v", err)
	}

	sawReadCall := false
	sawReadResult := false
	sawEditCall := false
	sawEditResult := false
	for _, ev := range events {
		data := activityEventData(t, ev)
		switch ev.Type {
		case "tool.called":
			if data["tool"] == "read" && data["path"] == targetFile {
				sawReadCall = true
			}
			if data["tool"] == "write" && data["operation"] == "edit" && data["path"] == targetFile {
				sawEditCall = true
			}
		case "tool.completed":
			if data["tool"] == "read" && data["path"] == targetFile {
				sawReadResult = true
			}
			if data["tool"] == "write" && data["operation"] == "edit" && data["path"] == targetFile {
				sawEditResult = true
			}
		}
	}

	if !sawReadCall {
		t.Fatal("expected a read tool.called event for the target file")
	}
	if !sawReadResult {
		t.Fatal("expected a read tool.completed event for the target file")
	}
	if !sawEditCall {
		t.Fatal("expected a write edit tool.called event for the target file")
	}
	if !sawEditResult {
		t.Fatal("expected a write edit tool.completed event for the target file")
	}
}

func activityEventData(t *testing.T, ev agentapi.ActivityRecord) map[string]string {
	t.Helper()

	if len(ev.Data) == 0 {
		return nil
	}
	var data map[string]string
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		return nil
	}
	return data
}

func requireStoredPlanViaClient(t *testing.T, client *agentclient.Client, prompts []string) *agentapi.ChatResponse {
	t.Helper()

	var last *agentapi.ChatResponse
	for _, prompt := range prompts {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		resp, err := client.Chat(ctx, agentapi.ChatRequest{
			Message: prompt,
			Mode:    "plan",
		})
		cancel()
		if err != nil {
			if isTransientPlanError(err) {
				t.Logf("transient plan chat error: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}
			t.Fatalf("plan chat failed: %v", err)
		}
		last = resp
		if resp.Warning == "" {
			return resp
		}
		t.Logf("plan warning: %s", resp.Warning)
	}

	if last == nil {
		t.Fatal("requireStoredPlanViaClient called without prompts")
	}
	t.Fatalf("expected stored plan without warning, got: %s", last.Warning)
	return nil
}

func isTransientPlanError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, " 429") ||
		strings.Contains(msg, "rate-limit") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "temporarily rate-limited")
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
