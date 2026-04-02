// Package activity provides persisted, queryable event history for agents.
//
// The Writer is an async subscriber to the EventBus that persists events
// to the KB and maintains an in-memory ring buffer for recent-event queries.
// It is the separation of concerns: EventBus handles real-time distribution,
// Activity handles persistence and history.
package activity

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/core/pkg/kb"
)

// Event aliases eventbus.Event for backward compatibility during migration.
type Event = eventbus.Event

// Writer is an async subscriber to EventBus that persists events to KB
// and maintains a ring buffer for recent-event queries.
type Writer struct {
	kb   kb.Store
	bus  *eventbus.Bus
	stop chan struct{}
	ch   <-chan Event

	mu    sync.RWMutex
	ring  []Event
	head  int
	count int
	cap   int
}

// NewWriter creates an Activity writer that subscribes to the EventBus.
// It runs a goroutine that reads events and persists them.
func NewWriter(bus *eventbus.Bus, kb kb.Store, ringCapacity int) *Writer {
	if ringCapacity <= 0 {
		ringCapacity = 1024
	}
	w := &Writer{
		kb:   kb,
		bus:  bus,
		stop: make(chan struct{}),
		ring: make([]Event, ringCapacity),
		cap:  ringCapacity,
	}
	return w
}

// Start begins the async persist loop.
func (w *Writer) Start() {
	w.ch = w.bus.Subscribe(4096)
	go w.run(w.ch)
}

// Stop drains remaining events and shuts down.
func (w *Writer) Stop() {
	close(w.stop)
	w.bus.Unsubscribe(w.ch)
}

func (w *Writer) run(ch <-chan Event) {
	for {
		select {
		case <-w.stop:
			// Drain remaining buffered events.
			for {
				select {
				case ev, ok := <-ch:
					if !ok {
						return
					}
					w.persist(ev)
				default:
					return
				}
			}
		case ev, ok := <-ch:
			if !ok {
				return
			}
			w.persist(ev)
		}
	}
}

func (w *Writer) persist(ev Event) {
	w.mu.Lock()
	// Write to ring buffer.
	idx := (w.head + w.count) % w.cap
	if w.count == w.cap {
		w.ring[w.head] = ev
		w.head = (w.head + 1) % w.cap
	} else {
		w.ring[idx] = ev
		w.count++
	}
	w.mu.Unlock()

	// Persist to KB asynchronously.
	if w.kb != nil && ev.SessionID != "" {
		key := fmt.Sprintf("%s/%s", ev.SessionID, ev.ID)
		if data, err := json.Marshal(ev); err == nil {
			w.kb.Set("activity", key, data)
		}
	}
}

// Recent returns the last n events from the ring buffer.
func (w *Writer) Recent(n int) []Event {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if n > w.count {
		n = w.count
	}
	out := make([]Event, n)
	start := (w.head + w.count - n) % w.cap
	for i := 0; i < n; i++ {
		out[i] = w.ring[(start+i)%w.cap]
	}
	return out
}

// History loads persisted events for a session from the KB.
// Returns nil if no KB is configured or no events exist.
func (w *Writer) History(sessionID string) ([]Event, error) {
	if w.kb == nil {
		return nil, nil
	}
	prefix := sessionID + "/"
	keys, err := w.kb.List("activity")
	if err != nil {
		return nil, err
	}
	var out []Event
	for _, k := range keys {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		data, err := w.kb.Get("activity", k)
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
