// Package activity provides a streaming event feed for agent activity.
//
// Events cover the full lifecycle: session start/end, message additions,
// tool calls, plan updates, step dispatch/completion, context compaction,
// and checkpoint saves.
//
// The Feed type is both a Sink (for the agent to emit events) and a Source
// (for consumers to subscribe via Go channels). Events are kept in a
// ring buffer for real-time streaming and persisted to KB for history replay.
package activity

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/quarkloop/core/pkg/kb"
)

// EventType identifies the kind of activity event.
type EventType string

const (
	SessionStarted   EventType = "session.started"
	SessionEnded     EventType = "session.ended"
	MessageAdded     EventType = "message.added"
	ToolCalled       EventType = "tool.called"
	ToolCompleted    EventType = "tool.completed"
	PlanCreated      EventType = "plan.created"
	PlanUpdated      EventType = "plan.updated"
	StepDispatched   EventType = "step.dispatched"
	StepCompleted    EventType = "step.completed"
	StepFailed       EventType = "step.failed"
	ContextCompacted EventType = "context.compacted"
	CheckpointSaved  EventType = "checkpoint.saved"
)

// Event is a single activity record.
type Event struct {
	ID        string      `json:"id"`
	SessionID string      `json:"session_id"`
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// Sink accepts events from the agent.
type Sink interface {
	Emit(event Event)
}

// Source delivers events to subscribers via Go channels.
type Source interface {
	Subscribe(filter ...EventType) <-chan Event
	Unsubscribe(ch <-chan Event)
}

// Feed implements both Sink and Source with an in-memory ring buffer
// and optional KB persistence for history replay.
type Feed struct {
	mu          sync.RWMutex
	ring        []Event
	head        int
	count       int
	cap         int
	subscribers map[chan Event]map[EventType]bool
	kb          kb.Store // nil = no persistence
}

// NewFeed creates a Feed with the given ring buffer capacity.
// Pass a non-nil kb.Store to enable event persistence.
func NewFeed(capacity int, k kb.Store) *Feed {
	if capacity <= 0 {
		capacity = 1024
	}
	return &Feed{
		ring:        make([]Event, capacity),
		cap:         capacity,
		subscribers: make(map[chan Event]map[EventType]bool),
		kb:          k,
	}
}

// Emit records an event and broadcasts it to subscribers.
func (f *Feed) Emit(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	f.mu.Lock()
	// Write to ring buffer.
	idx := (f.head + f.count) % f.cap
	if f.count == f.cap {
		// Buffer full — overwrite oldest.
		f.ring[f.head] = event
		f.head = (f.head + 1) % f.cap
	} else {
		f.ring[idx] = event
		f.count++
	}
	// Snapshot subscribers under lock.
	subs := make([]struct {
		ch     chan Event
		filter map[EventType]bool
	}, 0, len(f.subscribers))
	for ch, filter := range f.subscribers {
		subs = append(subs, struct {
			ch     chan Event
			filter map[EventType]bool
		}{ch, filter})
	}
	f.mu.Unlock()

	// Persist to KB (non-blocking — don't hold lock).
	if f.kb != nil && event.SessionID != "" {
		key := fmt.Sprintf("%s/%s", event.SessionID, event.ID)
		if data, err := json.Marshal(event); err == nil {
			f.kb.Set("activity", key, data)
		}
	}

	// Broadcast to matching subscribers.
	for _, sub := range subs {
		if len(sub.filter) > 0 && !sub.filter[event.Type] {
			continue
		}
		select {
		case sub.ch <- event:
		default:
			// Drop event if subscriber is slow — non-blocking.
		}
	}
}

// Subscribe returns a channel that receives events matching the given types.
// Pass no filter types to receive all events.
// The returned channel has a buffer of 64.
func (f *Feed) Subscribe(filter ...EventType) <-chan Event {
	ch := make(chan Event, 64)
	filterMap := make(map[EventType]bool, len(filter))
	for _, t := range filter {
		filterMap[t] = true
	}

	f.mu.Lock()
	f.subscribers[ch] = filterMap
	f.mu.Unlock()

	return ch
}

// Unsubscribe removes a subscriber channel. The channel is closed.
func (f *Feed) Unsubscribe(ch <-chan Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Find the matching send channel.
	for sendCh := range f.subscribers {
		if sendCh == ch {
			delete(f.subscribers, sendCh)
			close(sendCh)
			return
		}
	}
}

// Recent returns the last n events from the ring buffer.
func (f *Feed) Recent(n int) []Event {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if n > f.count {
		n = f.count
	}
	out := make([]Event, n)
	start := (f.head + f.count - n) % f.cap
	for i := 0; i < n; i++ {
		out[i] = f.ring[(start+i)%f.cap]
	}
	return out
}

// History loads persisted events for a session from the KB.
// Returns nil if no KB is configured or no events exist.
func (f *Feed) History(sessionID string) ([]Event, error) {
	if f.kb == nil {
		return nil, nil
	}
	prefix := sessionID + "/"
	keys, err := f.kb.List("activity")
	if err != nil {
		return nil, err
	}
	var out []Event
	for _, k := range keys {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		data, err := f.kb.Get("activity", k)
		if err != nil {
			continue
		}
		var ev Event
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	return out, nil
}
