// Package intervention provides a per-session message queue for mid-execution
// user course-correction. Messages are injected into the agent loop between
// tool calls or cycle phases, allowing users to steer a running agent.
package intervention

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Message is a user-injected intervention.
type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Content   string    `json:"content"`
	Priority  int       `json:"priority"` // 0 = normal, higher = more urgent
	CreatedAt time.Time `json:"created_at"`
}

// DrainMode controls how interventions are consumed.
type DrainMode string

const (
	Single DrainMode = "single" // consume one message per poll
	Drain  DrainMode = "drain"  // consume all queued messages
)

// Queue holds pending intervention messages for a session.
type Queue struct {
	mu       sync.Mutex
	messages []Message
	maxSize  int // default: 10
}

// New creates a Queue with the given max size.
func New(maxSize int) *Queue {
	if maxSize <= 0 {
		maxSize = 10
	}
	return &Queue{
		messages: make([]Message, 0, maxSize),
		maxSize:  maxSize,
	}
}

// Push adds an intervention message. Returns error if queue is full.
func (q *Queue) Push(msg Message) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.messages) >= q.maxSize {
		return fmt.Errorf("intervention queue full (%d)", q.maxSize)
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	q.messages = append(q.messages, msg)
	// Sort by priority descending so higher priority messages are consumed first.
	sort.SliceStable(q.messages, func(i, j int) bool {
		return q.messages[i].Priority > q.messages[j].Priority
	})
	return nil
}

// Poll returns the next message(s) based on drain mode.
// Returns nil if the queue is empty.
func (q *Queue) Poll(mode DrainMode) []Message {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.messages) == 0 {
		return nil
	}

	if mode == Single {
		msg := q.messages[0]
		q.messages = q.messages[1:]
		return []Message{msg}
	}

	// Drain mode: return all.
	msgs := make([]Message, len(q.messages))
	copy(msgs, q.messages)
	q.messages = q.messages[:0]
	return msgs
}

// Pending returns the number of queued messages.
func (q *Queue) Pending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.messages)
}

// Clear empties the queue and returns the drained messages.
func (q *Queue) Clear() []Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	msgs := q.messages
	q.messages = q.messages[:0]
	return msgs
}
