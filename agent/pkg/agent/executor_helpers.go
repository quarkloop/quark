package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/model"
)

// ─── Context-aware inference ─────────────────────────────────────────────────

// inferWithContext appends a user message to the AgentContext, builds the
// payload via ContextAdapter, calls gateway.InferRaw, appends the agent
// response, and auto-compacts if pressure is high.
func (e *Executor) inferWithContext(ctx context.Context, ac *llmctx.AgentContext, userMsg string) (*model.RawResponse, error) {
	if userMsg != "" {
		m, err := e.newUserMsg(userMsg)
		if err != nil {
			return nil, fmt.Errorf("build user msg: %w", err)
		}
		if err := ac.AppendMessage(ctx, m); err != nil {
			return nil, fmt.Errorf("append user msg: %w", err)
		}
	}

	adapter, err := e.adapterReg.Get(e.gateway.Provider())
	if err != nil {
		return nil, fmt.Errorf("adapter for %s: %w", e.gateway.Provider(), err)
	}
	ca := llmctx.NewContextAdapter(ac, adapter)

	payload, err := ca.BuildRequest(llmctx.RequestOptions{
		Model:     e.gateway.ModelName(),
		MaxTokens: e.gateway.MaxTokens(),
	})
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := e.gateway.InferRaw(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("gateway: %w", err)
	}

	agtMsg, err := e.newAgentMsg(AuthorAgent, resp.Content)
	if err == nil {
		ac.AppendMessage(ctx, agtMsg)
	}

	if ac.Pressure() >= llmctx.PressureHigh {
		log.Printf("executor: context pressure %s — auto-compacting", ac.Pressure())
		if err := ac.Compact(ctx); err != nil {
			log.Printf("executor: compact error: %v", err)
		}
	}

	return resp, nil
}

// ─── Message factories (Task 4: consolidate helpers) ─────────────────────────

// newUserMsg creates a new user text message with auto-generated ID.
func (e *Executor) newUserMsg(text string) (*llmctx.Message, error) {
	id, err := e.idGen.Next()
	if err != nil {
		return nil, err
	}
	author, _ := llmctx.NewAuthorID(AuthorUser)
	return llmctx.NewTextMessage(id, author, llmctx.UserAuthor, text, e.tc)
}

// newAgentMsg creates a new agent text message with auto-generated ID.
func (e *Executor) newAgentMsg(authorName, text string) (*llmctx.Message, error) {
	id, err := e.idGen.Next()
	if err != nil {
		return nil, err
	}
	author, _ := llmctx.NewAuthorID(authorName)
	return llmctx.NewTextMessage(id, author, llmctx.AgentAuthor, text, e.tc)
}

// ─── Snapshot persistence ────────────────────────────────────────────────────

// saveCheckpoint saves the supervisor context to KB for session resumption.
func (e *Executor) saveCheckpoint() {
	if e.supervisorCtx == nil || e.snapRepo == nil {
		return
	}
	snapID, err := e.idGen.Next()
	if err != nil {
		log.Printf("executor: checkpoint ID error: %v", err)
		return
	}
	snap := llmctx.SnapshotFromContext(snapID, e.supervisorCtx)
	if err := e.snapRepo.SaveLatestSnapshot(snap); err != nil {
		log.Printf("executor: checkpoint save error: %v", err)
	}
}

// ─── KB helpers ──────────────────────────────────────────────────────────────

// gatherArtifacts reads relevant KB entries to provide as worker context.
func (e *Executor) gatherArtifacts() string {
	var sb strings.Builder
	namespaces := []string{NSMemory, NSDocuments, NSArtifacts, NSNotes}
	for _, ns := range namespaces {
		keys, err := e.kb.List(ns)
		if err != nil || len(keys) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n[%s]\n", ns))
		for _, k := range keys {
			data, err := e.kb.Get(ns, k)
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
func (e *Executor) buildSupervisorSystemPrompt(state map[string]interface{}) string {
	if data, err := e.kb.Get(NSConfig, KeySupervisorPrompt); err == nil && len(data) > 0 {
		return string(data)
	}
	if e.supervisor.SystemPrompt != "" {
		if !strings.Contains(e.supervisor.SystemPrompt, "\n") {
			if data, err := os.ReadFile(e.supervisor.SystemPrompt); err == nil {
				return string(data)
			}
		}
		return e.supervisor.SystemPrompt
	}

	agents := []string{}
	for name := range e.agents {
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
		strings.Join(e.dispatcher.List(), ", "))
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
