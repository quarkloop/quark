// Package kb implements a per-space knowledge-base store backed by the
// JSONL collection store. Callers pass the absolute directory where the
// collection file should live; the supervisor's Store wires this up per
// space.
package kb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/quarkloop/supervisor/pkg/space/store"
)

// Store is the per-space KB API.
type Store interface {
	Get(namespace, key string) ([]byte, error)
	Set(namespace, key string, value []byte) error
	Delete(namespace, key string) error
	List(namespace string) ([]string, error)
	Close() error
}

// entry is the persisted shape of a KB record in the JSONL file.
type entry struct {
	ID    string `json:"id"`    // "namespace/key"
	Value []byte `json:"value"` // raw payload
}

type jsonlStore struct {
	col *store.Collection
}

// Open opens the KB collection at the given absolute directory.
func Open(dir string) (Store, error) {
	col, err := store.Open(dir, "kb")
	if err != nil {
		return nil, fmt.Errorf("kb: open store: %w", err)
	}
	return &jsonlStore{col: col}, nil
}

func (s *jsonlStore) Close() error { return s.col.Close() }

func (s *jsonlStore) Get(namespace, key string) ([]byte, error) {
	var e entry
	if err := s.col.GetInto(storeID(namespace, key), &e); err != nil {
		if store.IsNotFound(err) {
			return nil, fmt.Errorf("%s/%s: not found", namespace, key)
		}
		return nil, err
	}
	return e.Value, nil
}

func (s *jsonlStore) Set(namespace, key string, value []byte) error {
	return s.col.Set(entry{ID: storeID(namespace, key), Value: value})
}

func (s *jsonlStore) Delete(namespace, key string) error {
	if err := s.col.Delete(storeID(namespace, key)); err != nil {
		if store.IsNotFound(err) {
			return fmt.Errorf("%s/%s: not found", namespace, key)
		}
		return err
	}
	return nil
}

func (s *jsonlStore) List(namespace string) ([]string, error) {
	prefix := namespace + "/"
	var keys []string
	for _, raw := range s.col.List() {
		var e entry
		if err := json.Unmarshal(raw, &e); err != nil {
			continue
		}
		if strings.HasPrefix(e.ID, prefix) {
			keys = append(keys, e.ID[len(prefix):])
		}
	}
	return keys, nil
}

func storeID(namespace, key string) string {
	return namespace + "/" + key
}

// SplitID separates "namespace/key" into its parts.
func SplitID(id string) (namespace, key string) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 {
		return "", id
	}
	return parts[0], parts[1]
}

// ValidateID returns an error if id isn't formatted as "namespace/key".
func ValidateID(id string) error {
	ns, key := SplitID(id)
	if ns == "" || key == "" {
		return fmt.Errorf("invalid key %q — must be <namespace>/<key>", id)
	}
	return nil
}
