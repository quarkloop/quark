// Package events is the supervisor's fan-out bus for space-scoped signals
// consumed by agents (primarily over SSE).
package events

import (
	"encoding/json"
	"sync"
	"time"
)

// Kind identifies the event type.
type Kind string

const (
	SessionCreated   Kind = "session.created"
	SessionDeleted   Kind = "session.deleted"
	QuarkfileUpdated Kind = "quarkfile.updated"
	PluginInstalled  Kind = "plugin.installed"
	PluginRemoved    Kind = "plugin.removed"
	AgentShutdown    Kind = "agent.shutdown"
)

// Event is the wire format for a supervisor → agent signal.
type Event struct {
	Kind    Kind            `json:"kind"`
	Space   string          `json:"space"`
	Time    time.Time       `json:"time"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Bus is an in-memory fan-out of events scoped by space.
type Bus struct {
	mu   sync.RWMutex
	subs map[string]map[chan Event]struct{} // space → subscribers
}

// NewBus returns a fresh event bus.
func NewBus() *Bus {
	return &Bus{subs: make(map[string]map[chan Event]struct{})}
}

// Publish delivers e to every subscriber of e.Space. Slow subscribers are
// dropped; publish never blocks.
func (b *Bus) Publish(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs[e.Space] {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribe returns a channel receiving events for space. Call the returned
// function to unsubscribe.
func (b *Bus) Subscribe(space string) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	b.mu.Lock()
	if b.subs[space] == nil {
		b.subs[space] = make(map[chan Event]struct{})
	}
	b.subs[space][ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs[space], ch)
		b.mu.Unlock()
		close(ch)
	}
}

// Helpers to build typed payloads.

func SessionPayload(id string, kind string, title string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"id": id, "type": kind, "title": title})
	return b
}

func PluginPayload(name, typ string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"name": name, "type": typ})
	return b
}
