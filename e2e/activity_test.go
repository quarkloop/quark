//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	_ "github.com/quarkloop/agent-client"
)

func TestActivityEndpoint(t *testing.T) {
	if _, ok := cfgForTest(t, "OPENROUTER_API_KEY"); !ok {
		t.Skip("no provider configured")
	}
	client, stop := startAgentWithTools(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	records, err := client.Activity(ctx, 10)
	if err != nil {
		t.Fatalf("activity endpoint: %v", err)
	}
	t.Logf("got %d activity records", len(records))
}

func TestSessionsEndpoint(t *testing.T) {
	if _, ok := cfgForTest(t, "OPENROUTER_API_KEY"); !ok {
		t.Skip("no provider configured")
	}
	client, stop := startAgentWithTools(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sessions, err := client.Sessions(ctx)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	t.Logf("got %d sessions", len(sessions))
	if len(sessions) == 0 {
		t.Fatal("expected at least main session")
	}
	t.Logf("main session: key=%s type=%s", sessions[0].Key, sessions[0].Type)

	resp, err := client.CreateSession(ctx, agentapi.CreateSessionRequest{
		Type:  "chat",
		Title: "e2e-chat-test",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Logf("created session: key=%s", resp.Session.Key)
	if resp.Session.Key == "" {
		t.Fatal("expected session key")
	}

	sess, err := client.Session(ctx, resp.Session.Key)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.Key != resp.Session.Key {
		t.Errorf("session key mismatch")
	}

	if err := client.DeleteSession(ctx, resp.Session.Key); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	t.Log("session delete returned 204 (not checking persistence)")
}
