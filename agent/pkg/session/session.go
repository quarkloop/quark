// Package session tracks agent sessions — each session maps to a chat or
// autonomous run and links the agent's context to a conversation.
package session

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/quarkloop/core/pkg/kb"
)

// Status represents the lifecycle state of a session.
type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Session is a single agent run, mapping 1:1 with a chat conversation or
// an autonomous goal execution.
type Session struct {
	ID        string     `json:"id"`
	AgentRef  string     `json:"agent_ref"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	ChatID    string     `json:"chat_id,omitempty"`
	Status    Status     `json:"status"`
}

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
	return s.kb.Set(namespace, sess.ID, data)
}

// Get retrieves a session by ID.
func (s *Store) Get(id string) (*Session, error) {
	data, err := s.kb.Get(namespace, id)
	if err != nil {
		return nil, fmt.Errorf("session %s not found", id)
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

// List returns all sessions. Malformed entries are silently skipped.
func (s *Store) List() ([]*Session, error) {
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
		out = append(out, &sess)
	}
	return out, nil
}
