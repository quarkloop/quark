package agent

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/core/pkg/kb"
)

// SaveSessionSnapshot persists a session's context to KB.
func SaveSessionSnapshot(k kb.Store, idGen llmctx.IDGenerator, sessionKey string, ac *llmctx.AgentContext) error {
	if ac == nil {
		return nil
	}
	snapID, err := idGen.Next()
	if err != nil {
		return fmt.Errorf("snapshot ID: %w", err)
	}
	snap := llmctx.SnapshotFromContext(snapID, ac)
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	key := snapshotKey(sessionKey)
	return k.Set(agentcore.NSSnapshots, key, data)
}

// LoadSessionSnapshot restores a session's context from KB.
func LoadSessionSnapshot(k kb.Store, tc llmctx.TokenComputer, idGen llmctx.IDGenerator, sessionKey string) (*llmctx.AgentContext, error) {
	key := snapshotKey(sessionKey)
	data, err := k.Get(agentcore.NSSnapshots, key)
	if err != nil {
		return nil, fmt.Errorf("snapshot %s: not found", key)
	}
	var snap llmctx.ContextSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}

	compactor, err := buildCompactor()
	if err != nil {
		return nil, fmt.Errorf("compactor: %w", err)
	}

	ac, err := llmctx.ContextFromSnapshot(&snap, tc,
		llmctx.WithRestoreIDGenerator(idGen),
		llmctx.WithRestoreCompactor(compactor))
	if err != nil {
		return nil, fmt.Errorf("restore context: %w", err)
	}
	return ac, nil
}

func snapshotKey(sessionKey string) string {
	return sessionKey + "/latest"
}

func buildCompactor() (llmctx.Compactor, error) {
	pipeline, err := llmctx.NewPipelineCompactor(
		llmctx.NewWeightBasedCompactor(),
		llmctx.NewFIFOCompactor(),
	)
	if err != nil {
		return nil, err
	}
	return llmctx.NewThresholdCompactor(pipeline, agentcore.DefaultCompactionThreshold)
}

// saveCheckpoint is a convenience wrapper for the agent.
func (a *Agent) saveCheckpoint(sessionKey string) {
	a.mu.RLock()
	state, ok := a.sessions[sessionKey]
	a.mu.RUnlock()
	if !ok || state.Context == nil {
		return
	}
	if err := SaveSessionSnapshot(a.res.KB, a.res.IDGen, sessionKey, state.Context); err != nil {
		log.Printf("agent: checkpoint save error for %s: %v", sessionKey, err)
	}
}
