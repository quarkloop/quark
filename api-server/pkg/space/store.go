package space

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/quarkloop/store/pkg/store"
)

// Store defines the interface for persisting space records.
type Store interface {
	Save(sp *Space) error
	Get(id string) (*Space, error)
	List() ([]*Space, error)
	Delete(id string) error
	Close() error
}

// JSONLStore persists space records in a JSONL collection.
// The file lives at <dir>/spaces.jsonl.
type JSONLStore struct {
	col *store.Collection
}

// OpenStore opens (or creates) a JSONL-backed space store at the given directory.
func OpenStore(dir string) (Store, error) {
	col, err := store.Open(dir, "spaces")
	if err != nil {
		return nil, fmt.Errorf("space store: %w", err)
	}
	return &JSONLStore{col: col}, nil
}

func (s *JSONLStore) Close() error { return s.col.Close() }

// Save upserts a space record, updating UpdatedAt before writing.
func (s *JSONLStore) Save(sp *Space) error {
	sp.UpdatedAt = time.Now()
	return s.col.Set(sp)
}

// Get retrieves a space by ID.
func (s *JSONLStore) Get(id string) (*Space, error) {
	var sp Space
	if err := s.col.GetInto(id, &sp); err != nil {
		if store.IsNotFound(err) {
			return nil, fmt.Errorf("space %q: not found", id)
		}
		return nil, fmt.Errorf("space store get: %w", err)
	}
	return &sp, nil
}

// List returns all space records in insertion order.
func (s *JSONLStore) List() ([]*Space, error) {
	raws := s.col.List()
	spaces := make([]*Space, 0, len(raws))
	for _, raw := range raws {
		var sp Space
		if err := json.Unmarshal(raw, &sp); err != nil {
			continue // skip corrupt records
		}
		spaces = append(spaces, &sp)
	}
	return spaces, nil
}

// Delete removes the space record with the given ID.
func (s *JSONLStore) Delete(id string) error {
	if err := s.col.Delete(id); err != nil {
		if store.IsNotFound(err) {
			return fmt.Errorf("space %q: not found", id)
		}
		return fmt.Errorf("space store delete: %w", err)
	}
	return nil
}
