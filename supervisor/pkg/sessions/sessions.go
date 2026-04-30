// Package sessions is the supervisor-side session store. Sessions are owned
// by the supervisor and the agent is notified of create/delete events via the
// supervisor event stream.
package sessions

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned when a session does not exist.
var ErrNotFound = errors.New("session not found")

// Type is the kind of a session.
type Type string

const (
	TypeMain     Type = "main"
	TypeChat     Type = "chat"
	TypeSubAgent Type = "subagent"
	TypeCron     Type = "cron"
)

// Session is the canonical supervisor-side record for a conversation.
type Session struct {
	ID        string    `json:"id"`
	Space     string    `json:"space"`
	Type      Type      `json:"type"`
	Title     string    `json:"title,omitempty"`
	Status    string    `json:"status"` // active | archived
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store persists sessions for a single space as one JSONL file per session.
type Store struct {
	mu       sync.RWMutex
	dir      string
	space    string
	sessions map[string]*Session
}

// Open opens (or creates) the session store rooted at dir.
// Space is stamped onto each created session.
func Open(dir, space string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir session dir: %w", err)
	}
	s := &Store{dir: dir, space: space, sessions: make(map[string]*Session)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("read sessions dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		if err := s.loadFile(filepath.Join(s.dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) loadFile(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open session %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var sess Session
		if err := json.Unmarshal(line, &sess); err != nil {
			return fmt.Errorf("parse session %s: %w", filepath.Base(path), err)
		}
		if sess.Status == "" {
			sess.Status = "active"
		}
		s.sessions[sess.ID] = &sess
	}
	return scanner.Err()
}

func (s *Store) persistSessionLocked(sess *Session) error {
	path, err := s.sessionPath(sess.ID)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(sess); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) sessionPath(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\`) || filepath.Base(id) != id {
		return "", ErrNotFound
	}
	return filepath.Join(s.dir, id+".jsonl"), nil
}

// Create inserts a new session. The ID is generated.
func (s *Store) Create(t Type, title string) (*Session, error) {
	if t == "" {
		t = TypeChat
	}
	sess := &Session{
		ID:        newID(),
		Space:     s.space,
		Type:      t,
		Title:     title,
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
	if err := s.persistSessionLocked(sess); err != nil {
		delete(s.sessions, sess.ID)
		return nil, err
	}
	return sess, nil
}

// Get returns a session by id.
func (s *Store) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *sess
	return &cp, nil
}

// List returns all sessions.
func (s *Store) List() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		cp := *sess
		out = append(out, &cp)
	}
	return out
}

// Delete removes a session.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		return ErrNotFound
	}
	delete(s.sessions, id)
	path, err := s.sessionPath(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Touch updates the UpdatedAt timestamp.
func (s *Store) Touch(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return ErrNotFound
	}
	sess.UpdatedAt = time.Now().UTC()
	return s.persistSessionLocked(sess)
}

func newID() string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
