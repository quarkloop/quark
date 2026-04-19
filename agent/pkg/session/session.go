package session

import (
	"sync"
	"time"

	"github.com/quarkloop/agent/pkg/message"
)

// Session represents a communication channel with isolated context.
type Session struct {
	mu          sync.RWMutex
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Status      string            `json:"status"`
	Messages    []message.Message `json:"-"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	subscribers map[chan message.Message]struct{}
}

// AddMessage appends a message to the session history and notifies subscribers.
func (s *Session) AddMessage(role, content string) {
	msg := message.Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	s.mu.Lock()
	s.Messages = append(s.Messages, msg)
	subs := make([]chan message.Message, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subs = append(subs, ch)
	}
	s.mu.Unlock()

	// Notify outside lock
	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

// GetMessages returns a copy of the session message history.
func (s *Session) GetMessages() []message.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]message.Message, len(s.Messages))
	copy(out, s.Messages)
	return out
}

// Subscribe returns a channel that receives new messages added to the session.
func (s *Session) Subscribe() chan message.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan message.Message, 16)
	if s.subscribers == nil {
		s.subscribers = make(map[chan message.Message]struct{})
	}
	s.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscriber channel.
func (s *Session) Unsubscribe(ch chan message.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers, ch)
}

// Registry manages all sessions for an agent.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewRegistry creates a new session Registry.
func NewRegistry() *Registry {
	return &Registry{sessions: make(map[string]*Session)}
}

// GetOrCreate returns an existing session or creates a new one.
func (r *Registry) GetOrCreate(id, sessionType, title string) *Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	if s, ok := r.sessions[id]; ok {
		return s
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s := &Session{
		ID:        id,
		Type:      sessionType,
		Title:     title,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.sessions[id] = s
	return s
}

// Delete removes a session.
func (r *Registry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[id]; ok {
		s.Status = "ended"
		delete(r.sessions, id)
	}
}

// Get returns a session by ID.
func (r *Registry) Get(id string) *Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessions[id]
}

// Has returns true if a session with the given ID exists.
func (r *Registry) Has(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.sessions[id]
	return ok
}

// List returns all active sessions.
func (r *Registry) List() []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, s)
	}
	return out
}

// GetMessages returns the message history for a session.
func (r *Registry) GetMessages(id string) []message.Message {
	s := r.Get(id)
	if s == nil {
		return nil
	}
	return s.GetMessages()
}

// Subscribe subscribes to new messages on a session.
func (r *Registry) Subscribe(id string) chan message.Message {
	s := r.Get(id)
	if s == nil {
		return nil
	}
	return s.Subscribe()
}

// Unsubscribe removes a subscriber from a session.
func (r *Registry) Unsubscribe(id string, ch chan message.Message) {
	s := r.Get(id)
	if s == nil {
		return
	}
	s.Unsubscribe(ch)
}
