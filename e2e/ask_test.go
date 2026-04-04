//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	_ "github.com/quarkloop/agent-client"
)

func TestAskMode(t *testing.T) {
	if _, ok := cfgForTest(t, "OPENROUTER_API_KEY"); !ok {
		t.Skip("no provider configured")
	}
	t.Logf("provider configured, starting agent")
	client, stop := startAgentWithTools(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	t.Log("sending chat request")

	var resp *agentapi.ChatResponse
	var err error
	for i := 0; i < 5; i++ {
		resp, err = client.Chat(ctx, agentapi.ChatRequest{
			Message: "What is 2+2? Reply with just the number.",
			Mode:    "ask",
		})
		if err != nil && isRateLimit(err) {
			t.Logf("rate limited, retry %d/5", i+1)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	if err != nil {
		t.Fatalf("chat error: %v", err)
	}
	t.Logf("reply: %q", resp.Reply)
	t.Logf("mode: %q", resp.Mode)
	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}
	if resp.Mode != "ask" {
		t.Errorf("expected mode=ask, got: %s", resp.Mode)
	}
}
