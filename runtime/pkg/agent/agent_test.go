package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/runtime/pkg/pluginmanager"
)

func TestSystemPromptIncludesConfiguredAddenda(t *testing.T) {
	a := newTestAgent(t)
	a.config.PromptAddenda = []string{"", "Use service-backed tools for indexing."}

	got := a.systemPrompt()
	if !strings.Contains(got, "Use service-backed tools for indexing.") {
		t.Fatalf("system prompt missing addendum:\n%s", got)
	}
}

func TestDefaultToolsComesFromPluginManager(t *testing.T) {
	a := newTestAgent(t)
	a.Plugins.RegisterRuntimeTool(pluginmanager.RuntimeTool{
		Schema: plugin.ToolSchema{Name: "runtime_echo", Description: "echo"},
		Handler: func(context.Context, string) (string, error) {
			return "ok", nil
		},
	})

	tools := a.defaultTools()
	if len(tools) != 1 || tools[0].Name != "runtime_echo" {
		t.Fatalf("default tools = %+v", tools)
	}
}

func TestExecuteToolRoutesThroughPluginManager(t *testing.T) {
	a := newTestAgent(t)
	a.Plugins.RegisterRuntimeTool(pluginmanager.RuntimeTool{
		Schema: plugin.ToolSchema{Name: "runtime_echo", Description: "echo"},
		Handler: func(ctx context.Context, arguments string) (string, error) {
			if arguments != `{"value":"hello"}` {
				t.Fatalf("arguments = %s", arguments)
			}
			return "hello", nil
		},
	})

	got, err := a.executeTool(context.Background(), "runtime_echo", `{"value":"hello"}`)
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if got != "hello" {
		t.Fatalf("tool result = %q, want hello", got)
	}
}

func newTestAgent(t *testing.T) *Agent {
	t.Helper()
	a, err := NewAgent(Config{ID: "test-agent", PluginsDir: t.TempDir()})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}
	return a
}
