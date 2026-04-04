// Package kb implements the space knowledge base, backed by the unified JSONL
// store. Each KB entry is keyed by "namespace/key" and stored as a JSON record
// in a single kb.jsonl file inside the space directory.
package kb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/quarkloop/core/pkg/store"
)

// entry is the persisted shape of a KB record in the JSONL file.
type entry struct {
	ID    string `json:"id"`    // "namespace/key"
	Value []byte `json:"value"` // raw payload
}

// JSONLStore implements Store using the pkg/store JSONL Collection.
type JSONLStore struct {
	col *store.Collection
}

// Open opens (or creates) the KB for the given space directory.
// The collection is stored at <spaceDir>/kb/kb.jsonl.
func Open(spaceDir string) (Store, error) {
	col, err := store.Open(spaceDir+"/kb", "kb")
	if err != nil {
		return nil, fmt.Errorf("kb: open store: %w", err)
	}
	return &JSONLStore{col: col}, nil
}

func (s *JSONLStore) Close() error { return s.col.Close() }

// Get retrieves the value stored under namespace/key.
func (s *JSONLStore) Get(namespace, key string) ([]byte, error) {
	var e entry
	if err := s.col.GetInto(storeID(namespace, key), &e); err != nil {
		if store.IsNotFound(err) {
			return nil, fmt.Errorf("%s/%s: not found", namespace, key)
		}
		return nil, err
	}
	return e.Value, nil
}

// Set stores value under namespace/key, overwriting any previous value.
func (s *JSONLStore) Set(namespace, key string, value []byte) error {
	return s.col.Set(entry{ID: storeID(namespace, key), Value: value})
}

// Delete removes the entry at namespace/key.
func (s *JSONLStore) Delete(namespace, key string) error {
	if err := s.col.Delete(storeID(namespace, key)); err != nil {
		if store.IsNotFound(err) {
			return fmt.Errorf("%s/%s: not found", namespace, key)
		}
		return err
	}
	return nil
}

// List returns all keys within the given namespace.
func (s *JSONLStore) List(namespace string) ([]string, error) {
	prefix := namespace + "/"
	var keys []string
	for _, raw := range s.col.List() {
		var e entry
		if err := json.Unmarshal(raw, &e); err != nil {
			continue
		}
		if len(e.ID) > len(prefix) && e.ID[:len(prefix)] == prefix {
			keys = append(keys, e.ID[len(prefix):])
		}
	}
	return keys, nil
}

func storeID(namespace, key string) string {
	return namespace + "/" + key
}

func SplitID(id string) (namespace, key string) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 {
		return "", id
	}
	return parts[0], parts[1]
}

func ValidateID(id string) error {
	ns, key := SplitID(id)
	if ns == "" || key == "" {
		return fmt.Errorf("invalid key %q — must be <namespace>/<key>", id)
	}
	return nil
}
