// Package agent / snapshot.go provides KB-backed persistence for llmctx
// context snapshots, enabling session resumption across space-runtime restarts.
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/core/pkg/kb"
)

// KBSnapshotRepository implements context snapshot persistence backed by the
// space KB (Badger). Snapshots are stored in the "snapshots" namespace.
//
// The well-known key "supervisor-latest" is used for quick session resumption
// without listing all snapshots.
type KBSnapshotRepository struct {
	kb kb.Store
}

// NewKBSnapshotRepository creates a snapshot repository backed by the given KB store.
func NewKBSnapshotRepository(k kb.Store) *KBSnapshotRepository {
	return &KBSnapshotRepository{kb: k}
}

// SaveSnapshot persists a ContextSnapshot to the KB under its snapshot ID.
func (r *KBSnapshotRepository) SaveSnapshot(_ context.Context, snapshot *llmctx.ContextSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	return r.kb.Set(NSSnapshots, snapshot.SnapshotID.String(), data)
}

// LoadSnapshot retrieves a previously saved snapshot by its message ID.
// Returns an error if the snapshot does not exist.
func (r *KBSnapshotRepository) LoadSnapshot(_ context.Context, id llmctx.MessageID) (*llmctx.ContextSnapshot, error) {
	data, err := r.kb.Get(NSSnapshots, id.String())
	if err != nil {
		return nil, fmt.Errorf("snapshot %s: not found", id.String())
	}
	var snap llmctx.ContextSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	return &snap, nil
}

// ListSnapshots returns all saved snapshots. Malformed entries are silently skipped.
func (r *KBSnapshotRepository) ListSnapshots(_ context.Context) ([]*llmctx.ContextSnapshot, error) {
	keys, err := r.kb.List(NSSnapshots)
	if err != nil {
		return nil, err
	}
	var out []*llmctx.ContextSnapshot
	for _, k := range keys {
		data, err := r.kb.Get(NSSnapshots, k)
		if err != nil {
			continue
		}
		var snap llmctx.ContextSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}
		out = append(out, &snap)
	}
	return out, nil
}

// DeleteSnapshot removes a snapshot by its message ID.
func (r *KBSnapshotRepository) DeleteSnapshot(_ context.Context, id llmctx.MessageID) error {
	return r.kb.Delete(NSSnapshots, id.String())
}

// LoadLatestSnapshot loads the most recently saved supervisor context snapshot
// stored under the well-known key "supervisor-latest".
func (r *KBSnapshotRepository) LoadLatestSnapshot() (*llmctx.ContextSnapshot, error) {
	data, err := r.kb.Get(NSSnapshots, KeyLatestSnapshot)
	if err != nil {
		return nil, err
	}
	var snap llmctx.ContextSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	return &snap, nil
}

// SaveLatestSnapshot saves a snapshot under the well-known key "supervisor-latest"
// for fast session resumption. This is called at the end of every supervisor cycle.
func (r *KBSnapshotRepository) SaveLatestSnapshot(snapshot *llmctx.ContextSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	return r.kb.Set(NSSnapshots, KeyLatestSnapshot, data)
}
