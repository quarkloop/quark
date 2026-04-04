//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	agentapi "github.com/quarkloop/agent-api"
)

func TestMasterPlanMode(t *testing.T) {
	if _, ok := cfgForTest(t, "OPENROUTER_API_KEY"); !ok {
		t.Skip("no provider configured")
	}
	client, stop := startAgentWithTools(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var resp *agentapi.ChatResponse
	var err error
	for i := 0; i < 5; i++ {
		resp, err = client.Chat(ctx, agentapi.ChatRequest{
			Message: "Create a master plan with phases.",
			Mode:    "masterplan",
		})
		if err != nil && isRateLimit(err) {
			t.Logf("rate limited, retry %d/5", i+1)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}
}
