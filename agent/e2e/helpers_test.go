//go:build e2e

package e2e

import (
	"context"
	"os"
	"testing"

	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/skill"
	"github.com/quarkloop/core/pkg/kb"
)

// ─── Default test providers ────────────────────────────────────────────────────
// Hardcoded free-tier keys. Override via environment variables if needed.

const (
	defaultOpenRouterKey   = "[REDACTED_OPENROUTER_KEY]"
	defaultOpenRouterModel = "stepfun/step-3.5-flash:free"

	defaultZhipuKey   = "[REDACTED_ZHIPU_KEY]"
	defaultZhipuModel = "GLM-4.7-Flash"
)

// providerConfig holds resolved provider settings for a test run.
type providerConfig struct {
	provider string
	model    string
	apiKey   string
}

// resolveProvider returns the provider configuration for E2E tests.
// Priority: env var override → hardcoded defaults → OpenRouter first, Zhipu fallback.
func resolveProvider(t *testing.T) (provider, modelName, apiKey string) {
	t.Helper()
	cfg := resolveProviderConfig(t)
	return cfg.provider, cfg.model, cfg.apiKey
}

func resolveProviderConfig(t *testing.T) providerConfig {
	t.Helper()

	// 1. Environment override — explicit key takes priority.
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		m := os.Getenv("OPENROUTER_MODEL")
		if m == "" {
			m = defaultOpenRouterModel
		}
		return providerConfig{"openrouter", m, key}
	}
	if key := os.Getenv("ZHIPU_API_KEY"); key != "" {
		m := os.Getenv("ZHIPU_MODEL")
		if m == "" {
			m = defaultZhipuModel
		}
		return providerConfig{"zhipu", m, key}
	}

	// 2. Hardcoded defaults — try OpenRouter first.
	if defaultOpenRouterKey != "" {
		return providerConfig{"openrouter", defaultOpenRouterModel, defaultOpenRouterKey}
	}
	if defaultZhipuKey != "" {
		return providerConfig{"zhipu", defaultZhipuModel, defaultZhipuKey}
	}

	t.Skip("no API key available")
	return providerConfig{}
}

// ─── Agent builders ────────────────────────────────────────────────────────────

// testAgentOpts holds options for creating a test agent.
type testAgentOpts struct {
	mode           agent.Mode
	approvalPolicy agent.ApprovalPolicy
	provider       string
	modelName      string
	apiKey         string
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
	disp := skill.NewHTTPDispatcher()
	def := &agent.Definition{
		Ref:     "quark/test@latest",
		Name:    "test-agent",
		Version: "1.0.0",
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
