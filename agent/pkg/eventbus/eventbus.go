// Package eventbus provides a lightweight in-memory pub/sub event bus.
//
// Each subscriber gets its own buffered channel. Emit is non-blocking —
// events are dropped for slow subscribers rather than back-pressuring
// the publisher. There is no ring buffer and no persistence; late
// subscribers use the Activity store for catch-up.
package eventbus

import (
	"sync"
	"time"
)

// EventKind is a typed event discriminator for filtering.
type EventKind string

const (
	KindSessionStarted    EventKind = "session.started"
	KindSessionEnded      EventKind = "session.ended"
	KindMessageAdded      EventKind = "message.added"
	KindToolCalled        EventKind = "tool.called"
	KindToolCompleted     EventKind = "tool.completed"
	KindPlanCreated       EventKind = "plan.created"
	KindPlanUpdated       EventKind = "plan.updated"
	KindStepDispatched    EventKind = "step.dispatched"
	KindStepCompleted     EventKind = "step.completed"
	KindStepFailed        EventKind = "step.failed"
	KindContextCompacted  EventKind = "context.compacted"
	KindCheckpointSaved   EventKind = "checkpoint.saved"
	KindModeClassified    EventKind = "mode.classified"
	KindMasterPlanCreated EventKind = "masterplan.created"
	KindPhaseStarted      EventKind = "phase.started"
	KindPhaseCompleted    EventKind = "phase.completed"
	KindPhaseFailed       EventKind = "phase.failed"
	KindConfigChanged     EventKind = "config.changed"
	KindPluginLoaded      EventKind = "plugin.loaded"
	KindPluginUnloaded    EventKind = "plugin.unloaded"
	KindIntervention      EventKind = "intervention.received"
	KindBudgetSoftLimit   EventKind = "budget.soft_limit"
	KindBudgetHardLimit   EventKind = "budget.hard_limit"
	KindBudgetCompacted   EventKind = "budget.compacted"
)

// Event is the unit of broadcast.
type Event struct {
	ID        string
	Kind      EventKind
	SessionID string
	Timestamp time.Time
	Data      interface{}
}

// Bus is an in-memory pub/sub broadcaster.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[*subscriber]struct{}
}

type subscriber struct {
	ch     chan Event
	filter map[EventKind]bool // empty = all events
}

// New creates a Bus.
func New() *Bus {
	return &Bus{
		subscribers: make(map[*subscriber]struct{}),
	}
}

// Emit broadcasts an event to all matching subscribers. Non-blocking.
// Drop-on-full per subscriber — one slow subscriber doesn't block others.
func (b *Bus) Emit(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	b.mu.RLock()
	for s := range b.subscribers {
		if len(s.filter) > 0 && !s.filter[event.Kind] {
			continue
		}
		select {
		case s.ch <- event:
		default:
			// Drop event if subscriber is slow — non-blocking.
		}
	}
	b.mu.RUnlock()
}

// Subscribe returns a channel receiving events matching the given kinds.
// Pass no kinds to receive all events. Buffer size is configurable.
func (b *Bus) Subscribe(bufSize int, kinds ...EventKind) <-chan Event {
	ch := make(chan Event, bufSize)
	filter := make(map[EventKind]bool, len(kinds))
	for _, k := range kinds {
		filter[k] = true
	}
	s := &subscriber{ch: ch, filter: filter}

	b.mu.Lock()
	b.subscribers[s] = struct{}{}
	b.mu.Unlock()

	return ch
}

// Unsubscribe removes and closes the subscriber channel.
func (b *Bus) Unsubscribe(ch <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subscribers {
		if s.ch == ch {
			delete(b.subscribers, s)
			close(s.ch)
			return
		}
	}
}
