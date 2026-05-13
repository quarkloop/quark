package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/quarkloop/pkg/plugin"
)

func TestAgentUsesCapabilityProviderForPromptToolsAndDispatch(t *testing.T) {
	capability := fakeCapability{}
	a, err := NewAgent(Config{
		ID:            "main",
		ModelProvider: "noop",
		Model:         "noop/noop",
		Capabilities:  []CapabilityProvider{capability},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(a.systemPrompt(), "fake service prompt") {
		t.Fatalf("system prompt missing capability prompt:\n%s", a.systemPrompt())
	}

	tools := a.defaultTools()
	if len(tools) != 1 || tools[0].Name != "fake-service" {
		t.Fatalf("tools = %+v", tools)
	}

	out, err := a.executeTool(context.Background(), "fake-service", `{"ok":true}`)
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"handled":true}` {
		t.Fatalf("tool output = %q", out)
	}
}

type fakeCapability struct{}

func (fakeCapability) Prompt() string {
	return "fake service prompt"
}

func (fakeCapability) ToolSchemas() []plugin.ToolSchema {
	return []plugin.ToolSchema{{Name: "fake-service"}}
}

func (fakeCapability) ExecuteTool(ctx context.Context, name, arguments string) (string, bool, error) {
	if name != "fake-service" {
		return "", false, nil
	}
	return `{"handled":true}`, true, nil
}
