// Package event — integration test for the Receiver.
//
// Starts an in-process NATS server, registers a Receiver with a fake
// EventStore, publishes a NodeEvent on quark.data.event.<runtimeId>,
// and verifies the Receiver persists it.
package event

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/quarkloop/quark/server/internal/domain"
	"github.com/quarkloop/quark/server/internal/store"
	"go.uber.org/zap"
)

// startInProcessNATS starts a real NATS server in-process and returns
// a connected *nats.Conn. Both are cleaned up via t.Cleanup.
func startInProcessNATS(t *testing.T) *nats.Conn {
	t.Helper()
	opts := &server.Options{
		Host:          "127.0.0.1",
		Port:          -1,
		NoLog:         true,
		NoSigs:        true,
		WriteDeadline: 10 * time.Second,
	}
	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("nats-server NewServer: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatalf("nats-server did not become ready in 5s")
	}
	t.Cleanup(func() { s.Shutdown() })

	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatalf("connect to in-process NATS: %v", err)
	}
	t.Cleanup(func() { nc.Close() })
	return nc
}

// fakeEventStore is a minimal in-memory EventStore for testing.
type fakeEventStore struct {
	events []*domain.NodeEvent
}

func (f *fakeEventStore) AppendEvent(_ context.Context, e *domain.NodeEvent) error {
	f.events = append(f.events, e)
	return nil
}

func (f *fakeEventStore) AppendEvents(_ context.Context, events []*domain.NodeEvent) error {
	f.events = append(f.events, events...)
	return nil
}

func (f *fakeEventStore) QueryEvents(_ context.Context, _ *store.EventFilter) ([]*domain.NodeEvent, error) {
	return f.events, nil
}

func (f *fakeEventStore) CountEvents(_ context.Context, _ *store.EventFilter) (int64, error) {
	return int64(len(f.events)), nil
}

// Compile-time assertion that fakeEventStore implements store.EventStore.
var _ store.EventStore = (*fakeEventStore)(nil)

func TestReceiver_StartAndStop(t *testing.T) {
	nc := startInProcessNATS(t)
	r := New(zap.NewNop(), nc, &fakeEventStore{})
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.Stop()
}

func TestReceiver_PersistsEvents(t *testing.T) {
	nc := startInProcessNATS(t)
	st := &fakeEventStore{}
	r := New(zap.NewNop(), nc, st)
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Stop()

	// Publish a NodeEvent on quark.data.event.shared
	ev := domain.NodeEvent{
		ID:         "test-1",
		Kind:       domain.EventNodeCreated,
		NodeName:   "timer",
		SystemName: "monitor",
		Namespace:  "alice",
		Timestamp:  float64(1782000000), // epoch-seconds
		Payload:    map[string]any{"uri": "quark/time/schedule/timer:v1"},
	}
	payload, _ := json.Marshal(ev)

	err := nc.Publish("quark.data.event.shared", payload)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	nc.Flush()

	// Wait for the Receiver to process the message
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(st.events) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(st.events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(st.events))
	}
	got := st.events[0]
	if got.ID != "test-1" {
		t.Errorf("ID = %q, want test-1", got.ID)
	}
	if got.Kind != domain.EventNodeCreated {
		t.Errorf("Kind = %q, want NODE_CREATED", got.Kind)
	}
	// The receiver should have normalized the timestamp to a string
	if _, ok := got.Timestamp.(string); !ok {
		t.Errorf("Timestamp type = %T, want string (after normalization)", got.Timestamp)
	}
}

func TestReceiver_StopWithoutStart(t *testing.T) {
	// Stop() before Start() should be a no-op (not panic)
	nc := startInProcessNATS(t)
	r := New(zap.NewNop(), nc, &fakeEventStore{})
	r.Stop()
}

func TestReceiver_StopMultipleTimes(t *testing.T) {
	// Multiple Stop() calls should be safe
	nc := startInProcessNATS(t)
	r := New(zap.NewNop(), nc, &fakeEventStore{})
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.Stop()
	r.Stop()
	r.Stop()
}
