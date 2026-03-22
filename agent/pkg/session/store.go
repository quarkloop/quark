package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/quarkloop/core/pkg/kb"
)

const namespace = "sessions"

// Store persists sessions in the KB.
type Store struct {
	kb kb.Store
}

// NewStore creates a session store backed by the given KB.
func NewStore(k kb.Store) *Store {
	return &Store{kb: k}
}

// Create persists a new session.
func (s *Store) Create(sess *Session) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return s.kb.Set(namespace, sess.Key, data)
}

// Get retrieves a session by key.
func (s *Store) Get(key string) (*Session, error) {
	data, err := s.kb.Get(namespace, key)
	if err != nil {
		return nil, fmt.Errorf("session %s not found", key)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &sess, nil
}

// Update persists changes to an existing session.
func (s *Store) Update(sess *Session) error {
	return s.Create(sess) // same operation for KB
}

// Delete removes a session record from the KB.
func (s *Store) Delete(key string) error {
	return s.kb.Delete(namespace, key)
}

// ListByAgent returns all sessions for the given agent.
func (s *Store) ListByAgent(agentID string) ([]*Session, error) {
	prefix := fmt.Sprintf("agent:%s:", agentID)
	return s.listFiltered(func(sess *Session) bool {
		return strings.HasPrefix(sess.Key, prefix)
	})
}

// ListByType returns sessions of a specific type for the given agent.
func (s *Store) ListByType(agentID string, t Type) ([]*Session, error) {
	prefix := fmt.Sprintf("agent:%s:", agentID)
	return s.listFiltered(func(sess *Session) bool {
		return strings.HasPrefix(sess.Key, prefix) && sess.Type == t
	})
}

// ListActive returns all active sessions for the given agent.
func (s *Store) ListActive(agentID string) ([]*Session, error) {
	prefix := fmt.Sprintf("agent:%s:", agentID)
	return s.listFiltered(func(sess *Session) bool {
		return strings.HasPrefix(sess.Key, prefix) && sess.Status == StatusActive
	})
}

// GetMain returns the main session for the given agent.
func (s *Store) GetMain(agentID string) (*Session, error) {
	return s.Get(MainKey(agentID))
}

// MainExists checks whether a main session exists for the given agent.
func (s *Store) MainExists(agentID string) bool {
	_, err := s.Get(MainKey(agentID))
	return err == nil
}

// listFiltered returns all sessions matching the predicate.
// Malformed entries are silently skipped.
func (s *Store) listFiltered(pred func(*Session) bool) ([]*Session, error) {
	keys, err := s.kb.List(namespace)
	if err != nil {
		return nil, err
	}
	var out []*Session
	for _, k := range keys {
		data, err := s.kb.Get(namespace, k)
		if err != nil {
			continue
		}
		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}
		if pred == nil || pred(&sess) {
			out = append(out, &sess)
		}
	}
	return out, nil
}
