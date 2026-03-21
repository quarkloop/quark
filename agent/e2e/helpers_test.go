//go:build e2e

package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/agent/pkg/tool"
	"github.com/quarkloop/core/pkg/kb"
	bashtool "github.com/quarkloop/tools/bash/pkg/bash"
	readtool "github.com/quarkloop/tools/read/pkg/read"
	writetool "github.com/quarkloop/tools/write/pkg/write"
)

// loadDotEnv reads KEY=VALUE pairs from a .env file and sets them as
// environment variables (without overwriting already-set vars).
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // file absent — silently skip
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func init() {
	// Load .env files relative to this source file.
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile is quark/agent/e2e/helpers_test.go.
	quarkRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	workspaceRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")

	// Prefer quark/.env, then fall back to the workspace root .env.
	loadDotEnv(filepath.Join(quarkRoot, ".env"))
	loadDotEnv(filepath.Join(workspaceRoot, ".env"))
}

// providerConfig holds resolved provider settings for a test run.
type providerConfig struct {
	provider string
	model    string
	apiKey   string
}

// resolveProvider returns the provider configuration for E2E tests.
// Priority: OPENROUTER_API_KEY → ZHIPU_API_KEY (both loaded from .env by init).
func resolveProvider(t *testing.T) (provider, modelName, apiKey string) {
	t.Helper()
	cfg := resolveProviderConfig(t)
	return cfg.provider, cfg.model, cfg.apiKey
}

func resolveProviderConfig(t *testing.T) providerConfig {
	t.Helper()

	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		m := firstEnv("OPENROUTER_E2E_MODEL", "OPENROUTER_MODEL")
		if m == "" {
			m = "stepfun/step-3.5-flash:free"
		}
		return providerConfig{"openrouter", m, key}
	}
	if key := os.Getenv("ZHIPU_API_KEY"); key != "" {
		m := firstEnv("ZHIPU_E2E_MODEL", "ZHIPU_MODEL")
		if m == "" {
			m = "GLM-4.7-Flash"
		}
		return providerConfig{"zhipu", m, key}
	}

	t.Skip("no API key available — set OPENROUTER_API_KEY or ZHIPU_API_KEY in .env")
	return providerConfig{}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// ─── Agent builders ────────────────────────────────────────────────────────────

// testAgentOpts holds options for creating a test agent.
type testAgentOpts struct {
	mode           agent.Mode
	approvalPolicy agent.ApprovalPolicy
	provider       string
	modelName      string
	apiKey         string
	name           string
	systemPrompt   string
	configureDisp  func(*tool.HTTPDispatcher)
}

type liveToolHarness struct {
	agent *agent.Agent
	kb    kb.Store
	feed  *activity.Feed
}

// newTestAgent creates a fully initialised Agent backed by a temp KB.
func newTestAgent(t *testing.T, mode agent.Mode) (*agent.Agent, kb.Store) {
	t.Helper()
	cfg := resolveProviderConfig(t)
	a, k, _ := buildTestAgent(t, testAgentOpts{
		mode:     mode,
		provider: cfg.provider, modelName: cfg.model, apiKey: cfg.apiKey,
	})
	return a, k
}

// newTestAgentWithFeed creates an Agent with an activity.Feed attached.
func newTestAgentWithFeed(t *testing.T, mode agent.Mode, ap agent.ApprovalPolicy) (*agent.Agent, kb.Store, *activity.Feed) {
	t.Helper()
	cfg := resolveProviderConfig(t)
	return buildTestAgent(t, testAgentOpts{
		mode:           mode,
		approvalPolicy: ap,
		provider:       cfg.provider, modelName: cfg.model, apiKey: cfg.apiKey,
	})
}

// newNoopAgentWithFeed creates an Agent backed by the noop gateway (no LLM calls).
func newNoopAgentWithFeed(t *testing.T, mode agent.Mode, ap agent.ApprovalPolicy) (*agent.Agent, kb.Store, *activity.Feed) {
	t.Helper()
	return buildTestAgent(t, testAgentOpts{
		mode:           mode,
		approvalPolicy: ap,
		provider:       "noop", modelName: "noop", apiKey: "",
	})
}

func buildTestAgent(t *testing.T, opts testAgentOpts) (*agent.Agent, kb.Store, *activity.Feed) {
	t.Helper()

	dir := t.TempDir()
	k, err := kb.Open(dir)
	if err != nil {
		t.Fatalf("open kb: %v", err)
	}
	t.Cleanup(func() { k.Close() })

	gw, err := model.New(model.GatewayConfig{
		Provider: opts.provider,
		Model:    opts.modelName,
		APIKey:   opts.apiKey,
	})
	if err != nil {
		t.Fatalf("create gateway: %v", err)
	}

	feed := activity.NewFeed(256, k)
	disp := tool.NewHTTPDispatcher()
	if opts.configureDisp != nil {
		opts.configureDisp(disp)
	}
	name := opts.name
	if name == "" {
		name = "test-agent"
	}
	def := &agent.Definition{
		Ref:          "quark/test@latest",
		Name:         name,
		Version:      "1.0.0",
		SystemPrompt: opts.systemPrompt,
		Config: agent.Config{
			ContextWindow:  4096,
			Compaction:     "sliding",
			MemoryPolicy:   "summarize",
			ApprovalPolicy: opts.approvalPolicy,
		},
		Capabilities: agent.Capabilities{
			SpawnAgents: true,
			MaxWorkers:  5,
			CreatePlans: true,
		},
	}

	a := agent.NewAgent(def, k, gw, disp,
		agent.WithMode(opts.mode),
		agent.WithActivitySink(feed),
	)
	if err := a.InitContext(def.Config.ContextWindow); err != nil {
		t.Fatalf("init context: %v", err)
	}

	return a, k, feed
}

func newLiveBashHarness(t *testing.T, ap agent.ApprovalPolicy) *liveToolHarness {
	t.Helper()

	cfg := resolveProviderConfig(t)

	bashServer := httptest.NewServer(bashtool.RunHandler())
	t.Cleanup(bashServer.Close)

	a, k, feed := buildTestAgent(t, testAgentOpts{
		mode:           agent.ModePlan,
		approvalPolicy: ap,
		provider:       cfg.provider,
		modelName:      cfg.model,
		apiKey:         cfg.apiKey,
		name:           "supervisor",
		systemPrompt: "You are a careful worker running an end-to-end bash tool test.\n\n" +
			"You have access to the \"bash\" tool.\n" +
			"When you need to execute a command, respond with only this fenced JSON block:\n\n" +
			"```tool\n" +
			"{\"name\":\"bash\",\"input\":{\"cmd\":\"<command>\"}}\n" +
			"```\n\n" +
			"Rules:\n" +
			"- For execution tasks, your first response must be a bash tool call.\n" +
			"- Use only the bash tool for shell and file-system work.\n" +
			"- Run exactly one shell command per tool call.\n" +
			"- If the task provides exact commands, use them verbatim.\n" +
			"- Do not invent command results.\n" +
			"- Do not say the task is complete until after you have made the required bash tool calls and seen their tool results.\n" +
			"- After the required bash calls are done, return a short final summary.",
		configureDisp: func(disp *tool.HTTPDispatcher) {
			disp.Register("bash", &tool.Definition{
				Ref:      "quark/bash@test",
				Name:     "bash",
				Version:  "1.0.0",
				Endpoint: bashServer.URL + "/run",
			})
		},
	})

	return &liveToolHarness{agent: a, kb: k, feed: feed}
}

func newLiveFileToolsHarness(t *testing.T, ap agent.ApprovalPolicy) *liveToolHarness {
	t.Helper()

	cfg := resolveProviderConfig(t)

	writeServer := httptest.NewServer(writetool.RunHandler())
	t.Cleanup(writeServer.Close)

	readServer := httptest.NewServer(readtool.RunHandler())
	t.Cleanup(readServer.Close)

	a, k, feed := buildTestAgent(t, testAgentOpts{
		mode:           agent.ModePlan,
		approvalPolicy: ap,
		provider:       cfg.provider,
		modelName:      cfg.model,
		apiKey:         cfg.apiKey,
		name:           "supervisor",
		systemPrompt: "You are a careful worker running an end-to-end file tool test.\n\n" +
			"You have access to the \"read\" tool and the \"write\" tool.\n" +
			"When you need to inspect a file, respond with only this fenced JSON block:\n\n" +
			"```tool\n" +
			"{\"name\":\"read\",\"input\":{\"path\":\"<path>\"}}\n" +
			"```\n\n" +
			"For reading a specific line range, you may use:\n\n" +
			"```tool\n" +
			"{\"name\":\"read\",\"input\":{\"path\":\"<path>\",\"start_line\":1,\"end_line\":20}}\n" +
			"```\n\n" +
			"When you need to write or update a file, respond with only this fenced JSON block:\n\n" +
			"```tool\n" +
			"{\"name\":\"write\",\"input\":{\"path\":\"<path>\",\"operation\":\"write\",\"content\":\"<text>\"}}\n" +
			"```\n\n" +
			"For precise code edits, use operation \"edit\" with 1-based line and column positions where the end position is exclusive, for example:\n\n" +
			"```tool\n" +
			"{\"name\":\"write\",\"input\":{\"path\":\"<path>\",\"operation\":\"edit\",\"edits\":[{\"start_line\":1,\"start_column\":1,\"end_line\":1,\"end_column\":1,\"new_text\":\"<replacement>\"}]}}\n" +
			"```\n\n" +
			"Rules:\n" +
			"- Use the read tool for file inspection and the write tool for file creation or updates.\n" +
			"- For file creation tasks, your first response must be a write tool call.\n" +
			"- For update tasks on an existing file, inspect it with the read tool before making any write call.\n" +
			"- Use the write tool for all file creation and text updates.\n" +
			"- A read call alone does not complete an update task; after inspection you must make the required write call and wait for its real result before any summary.\n" +
			"- When updating an existing code file, prefer the write tool edit operation over rewriting the whole file.\n" +
			"- When writing code, produce valid multi-line source code with proper newlines and indentation; do not collapse block-structured code into a one-line statement.\n" +
			"- Run exactly one read or write operation per tool call.\n" +
			"- If the task provides exact input objects, use them verbatim.\n" +
			"- Do not invent tool results or final file contents.\n" +
			"- Never claim a file was created or updated unless you have received a successful write tool result for that file.\n" +
			"- Never claim a file was inspected unless you have received a successful read tool result for that file.\n" +
			"- Do not say the task is complete until after you have made the required read and write calls and seen their tool results.\n" +
			"- After the required tool calls are done, return a short final summary.",
		configureDisp: func(disp *tool.HTTPDispatcher) {
			disp.Register("read", &tool.Definition{
				Ref:      "quark/read@test",
				Name:     "read",
				Version:  "1.0.0",
				Endpoint: readServer.URL + "/read",
			})
			disp.Register("write", &tool.Definition{
				Ref:      "quark/write@test",
				Name:     "write",
				Version:  "1.0.0",
				Endpoint: writeServer.URL + "/apply",
			})
		},
	})

	return &liveToolHarness{agent: a, kb: k, feed: feed}
}

// ─── Test utilities ────────────────────────────────────────────────────────────

// chat sends a ChatRequest and returns the response, failing the test on error.
func chat(t *testing.T, a *agent.Agent, message, mode string) *agent.ChatResponse {
	t.Helper()
	resp, err := a.Chat(context.Background(), agent.ChatRequest{
		Message: message,
		Mode:    mode,
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	return resp
}

// drainEvents reads all buffered events from a subscription channel.
func drainEvents(ch <-chan activity.Event) []activity.Event {
	var out []activity.Event
	for {
		select {
		case ev := <-ch:
			out = append(out, ev)
		default:
			return out
		}
	}
}

// hasEvent returns true if any event in the slice matches the given type.
func hasEvent(events []activity.Event, t activity.EventType) bool {
	for _, ev := range events {
		if ev.Type == t {
			return true
		}
	}
	return false
}

// eventData extracts the string map data from an event, or nil.
func eventData(ev activity.Event) map[string]string {
	if m, ok := ev.Data.(map[string]string); ok {
		return m
	}
	return nil
}

func dumpRecentActivity(t *testing.T, feed *activity.Feed, n int) {
	t.Helper()

	for _, ev := range feed.Recent(n) {
		data := eventData(ev)
		switch ev.Type {
		case activity.ToolCalled:
			t.Logf("activity %s step=%s tool=%s cmd=%s path=%s operation=%s start_line=%s end_line=%s args=%s",
				ev.Type, data["step"], data["tool"], data["cmd"], data["path"], data["operation"], data["start_line"], data["end_line"], data["args"])
		case activity.ToolCompleted:
			t.Logf("activity %s step=%s tool=%s path=%s operation=%s is_error=%s exit_code=%s bytes_read=%s bytes_written=%s replacements=%s edits_applied=%s total_lines=%s start_line=%s end_line=%s content=%s output=%s preview=%s result=%s error=%s",
				ev.Type, data["step"], data["tool"], data["path"], data["operation"], data["is_error"], data["exit_code"], data["bytes_read"], data["bytes_written"], data["replacements"], data["edits_applied"], data["total_lines"], data["start_line"], data["end_line"], data["content"], data["output"], data["content_preview"], data["result"], data["error"])
		default:
			t.Logf("activity %s data=%v", ev.Type, ev.Data)
		}
	}
}

func silenceStdLogger(t *testing.T) func() {
	t.Helper()

	prevWriter := log.Writer()
	prevFlags := log.Flags()
	prevPrefix := log.Prefix()

	log.SetOutput(io.Discard)
	log.SetFlags(0)
	log.SetPrefix("")

	return func() {
		log.SetOutput(prevWriter)
		log.SetFlags(prevFlags)
		log.SetPrefix(prevPrefix)
	}
}

func planSingleStepBashTask(t *testing.T, a *agent.Agent, commands []string, extraRules string) {
	t.Helper()

	n := len(commands)
	if n == 0 {
		t.Fatal("planSingleStepBashTask requires at least one command")
	}
	commandList := numberedCommands(commands)
	trimmedRules := strings.TrimSpace(extraRules)
	if trimmedRules != "" {
		trimmedRules = "\n" + trimmedRules
	}

	planPrompt := fmt.Sprintf(
		"Create a plan with one supervisor step. "+
			"That step must use the bash tool exactly %d times in %d separate calls, one command per call. "+
			"The exact commands are:\n%s%s",
		n, n, commandList, trimmedRules,
	)

	resp := requireStoredPlan(t, a, []string{
		planPrompt,
		fmt.Sprintf(
			"Retry the one-step supervisor plan. Use bash exactly %d times in %d separate calls with these commands:\n%s%s",
			n, n, commandList, trimmedRules,
		),
		fmt.Sprintf(
			"Create a fresh one-step supervisor plan. Use bash exactly %d times in %d separate calls with these exact commands:\n%s%s",
			n, n, commandList, trimmedRules,
		),
	})
	if resp.Mode != "plan" {
		t.Fatalf("expected mode=plan, got: %s", resp.Mode)
	}

	refinePrompt := fmt.Sprintf(
		"Revise the current plan so the single supervisor step says the first response must be a bash tool call and that bash must be used exactly %d times in %d separate calls. "+
			"The exact commands are:\n%s%s",
		n, n, commandList, trimmedRules,
	)

	requireStoredPlan(t, a, []string{
		refinePrompt,
		fmt.Sprintf(
			"Update the current plan. Keep one supervisor step. The first execution response must be a bash tool call. Use bash exactly %d times in %d separate calls with these exact commands:\n%s%s",
			n, n, commandList, trimmedRules,
		),
		fmt.Sprintf(
			"Revise the current one-step plan. The supervisor must use bash first and must make exactly %d separate bash calls with these commands:\n%s%s",
			n, commandList, trimmedRules,
		),
	})
}

func numberedCommands(commands []string) string {
	var b strings.Builder
	for i, cmd := range commands {
		fmt.Fprintf(&b, "%d. %s\n", i+1, cmd)
	}
	return strings.TrimRight(b.String(), "\n")
}

func requireStoredPlan(t *testing.T, a *agent.Agent, prompts []string) *agent.ChatResponse {
	t.Helper()

	var last *agent.ChatResponse
	for _, prompt := range prompts {
		resp := chat(t, a, prompt, "plan")
		last = resp
		if resp.Warning == "" && resp.Reply != "" {
			return resp
		}
	}

	if last == nil {
		t.Fatal("requireStoredPlan called without prompts")
	}
	t.Fatalf("expected stored plan without warning, got: %s", last.Warning)
	return nil
}

func waitForPlanCompletion(t *testing.T, k kb.Store, timeout time.Duration) *plan.Plan {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := k.Get("plans", "master")
		if err == nil {
			var p plan.Plan
			if err := json.Unmarshal(data, &p); err == nil {
				if p.Complete {
					return &p
				}
				for _, step := range p.Steps {
					if step.Status == plan.StepFailed {
						t.Fatalf("plan step failed: %s", mustJSON(t, &p))
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	data, err := k.Get("plans", "master")
	if err != nil {
		t.Fatalf("timed out waiting for plan completion and could not load plan: %v", err)
	}
	var p plan.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("timed out waiting for plan completion and could not decode plan: %v", err)
	}
	t.Fatalf("timed out waiting for plan completion: %s", mustJSON(t, &p))
	return nil
}

func runPlanToCompletion(t *testing.T, a *agent.Agent, k kb.Store, runTimeout, waitTimeout time.Duration) *plan.Plan {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- a.Run(ctx)
	}()

	finalPlan := waitForPlanCompletion(t, k, waitTimeout)
	cancel()

	runErr := <-runDone
	if runErr != nil && runErr != context.Canceled && runErr != context.DeadlineExceeded {
		t.Fatalf("run failed: %v", runErr)
	}

	return finalPlan
}

func loadPlan(t *testing.T, k kb.Store) *plan.Plan {
	t.Helper()

	data, err := k.Get("plans", "master")
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}
	var p plan.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	return &p
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal debug JSON: %v", err)
	}
	return string(data)
}
