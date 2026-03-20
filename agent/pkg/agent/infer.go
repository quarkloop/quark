package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/model"
)

// ─── Context-aware inference ─────────────────────────────────────────────────

// inferWithContext appends a user message to the AgentContext, builds the
// payload via ContextAdapter, calls gateway.InferRaw, appends the agent
// response, and auto-compacts if pressure is high.
func (a *Agent) inferWithContext(ctx context.Context, ac *llmctx.AgentContext, userMsg string) (*model.RawResponse, error) {
	if userMsg != "" {
		m, err := a.newUserMsg(userMsg)
		if err != nil {
			return nil, fmt.Errorf("build user msg: %w", err)
		}
		if err := ac.AppendMessage(ctx, m); err != nil {
			return nil, fmt.Errorf("append user msg: %w", err)
		}
	}

	adapter, err := a.adapterReg.Get(a.gateway.Provider())
	if err != nil {
		return nil, fmt.Errorf("adapter for %s: %w", a.gateway.Provider(), err)
	}
	ca := llmctx.NewContextAdapter(ac, adapter)

	payload, err := ca.BuildRequest(llmctx.RequestOptions{
		Model:     a.gateway.ModelName(),
		MaxTokens: a.gateway.MaxTokens(),
	})
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := a.gateway.InferRaw(ctx, payload)
	if err != nil {
		resp, err = a.inferWithRetry(ctx, payload, err)
		if err != nil {
			return nil, fmt.Errorf("gateway: %w", err)
		}
	}

	agtMsg, err := a.newAgentMsg(AuthorAgent, resp.Content)
	if err == nil {
		ac.AppendMessage(ctx, agtMsg)
	}

	if ac.Pressure() >= llmctx.PressureHigh {
		log.Printf("agent: context pressure %s — auto-compacting", ac.Pressure())
		if err := ac.Compact(ctx); err != nil {
			log.Printf("agent: compact error: %v", err)
		}
	}

	return resp, nil
}

func (a *Agent) inferWithRetry(ctx context.Context, payload []byte, firstErr error) (*model.RawResponse, error) {
	err := firstErr
	backoff := 2 * time.Second
	for attempt := 1; attempt <= 2; attempt++ {
		if !isRetryableGatewayError(err) {
			return nil, err
		}
		log.Printf("agent: retrying transient gateway error (attempt %d/2): %v", attempt, err)
		if sleepErr := sleepWithContext(ctx, backoff); sleepErr != nil {
			return nil, err
		}

		resp, retryErr := a.gateway.InferRaw(ctx, payload)
		if retryErr == nil {
			return resp, nil
		}
		err = retryErr
		backoff *= 2
	}
	return nil, err
}

func isRetryableGatewayError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, " 429") ||
		strings.Contains(msg, "rate-limit") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "temporarily rate-limited")
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ─── Message factories ───────────────────────────────────────────────────────

// newUserMsg creates a new user text message with auto-generated ID.
func (a *Agent) newUserMsg(text string) (*llmctx.Message, error) {
	id, err := a.idGen.Next()
	if err != nil {
		return nil, err
	}
	author, _ := llmctx.NewAuthorID(AuthorUser)
	return llmctx.NewTextMessage(id, author, llmctx.UserAuthor, text, a.tc)
}

// newAgentMsg creates a new agent text message with auto-generated ID.
func (a *Agent) newAgentMsg(authorName, text string) (*llmctx.Message, error) {
	id, err := a.idGen.Next()
	if err != nil {
		return nil, err
	}
	author, _ := llmctx.NewAuthorID(authorName)
	return llmctx.NewTextMessage(id, author, llmctx.AgentAuthor, text, a.tc)
}

// ─── Snapshot persistence ────────────────────────────────────────────────────

// saveCheckpoint saves the agent context to KB for session resumption.
func (a *Agent) saveCheckpoint() {
	if a.ctx == nil || a.snapRepo == nil {
		return
	}
	snapID, err := a.idGen.Next()
	if err != nil {
		log.Printf("agent: checkpoint ID error: %v", err)
		return
	}
	snap := llmctx.SnapshotFromContext(snapID, a.ctx)
	if err := a.snapRepo.SaveLatestSnapshot(snap); err != nil {
		log.Printf("agent: checkpoint save error: %v", err)
	}
}

// ─── KB helpers ──────────────────────────────────────────────────────────────

// gatherArtifacts reads relevant KB entries to provide as worker context.
func (a *Agent) gatherArtifacts() string {
	var sb strings.Builder
	namespaces := []string{NSMemory, NSDocuments, NSArtifacts, NSNotes}
	for _, ns := range namespaces {
		keys, err := a.kb.List(ns)
		if err != nil || len(keys) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n[%s]\n", ns))
		for _, k := range keys {
			data, err := a.kb.Get(ns, k)
			if err != nil {
				continue
			}
			sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", k, truncate(string(data), 1000)))
		}
	}
	return sb.String()
}

// ─── Prompt builders ─────────────────────────────────────────────────────────

// buildSupervisorSystemPrompt resolves the supervisor's system prompt from
// KB config, file path, inline text, or a generated default.
func (a *Agent) buildSupervisorSystemPrompt(state map[string]interface{}) string {
	if data, err := a.kb.Get(NSConfig, KeySupervisorPrompt); err == nil && len(data) > 0 {
		return string(data)
	}
	if a.def.SystemPrompt != "" {
		if !strings.Contains(a.def.SystemPrompt, "\n") {
			if data, err := os.ReadFile(a.def.SystemPrompt); err == nil {
				return string(data)
			}
		}
		return a.def.SystemPrompt
	}

	agents := []string{}
	for name := range a.subAgents {
		agents = append(agents, name)
	}
	return fmt.Sprintf(`You are the supervisor agent orchestrating a multi-agent space.

Available worker agents: %s
Available skills: %s

Your job is to:
1. Break the goal into concrete, parallelizable steps
2. Assign each step to the most appropriate agent
3. Track progress and adjust the plan as needed
4. Declare completion when the goal is fully achieved

Always respond with valid JSON as instructed.`,
		strings.Join(agents, ", "),
		strings.Join(a.dispatcher.List(), ", "))
}

// ─── Utility ─────────────────────────────────────────────────────────────────

// truncate shortens a string for logging.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// extractJSON pulls the first JSON object from a string (handles markdown fences).
func extractJSON(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		end := strings.LastIndex(s, "```")
		if end > 3 {
			s = strings.TrimSpace(s[3:end])
			if nl := strings.Index(s, "\n"); nl >= 0 {
				s = strings.TrimSpace(s[nl:])
			}
		}
	}
	start := strings.Index(s, "{")
	if start < 0 {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				raw := []byte(s[start : i+1])
				if json.Valid(raw) {
					return raw, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("malformed JSON in response")
}
