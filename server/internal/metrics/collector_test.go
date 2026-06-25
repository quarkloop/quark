package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// startInProcessNATS starts a real NATS server in-process, returns
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

func TestCollector_HeartbeatHandling(t *testing.T) {
	nc := startInProcessNATS(t)
	c := New(testLogger(), nc)

	// Inject a snapshot directly (bypasses NATS) — simulates a heartbeat
	c.SnapshotForTesting(context.Background(), "alice", Snapshot{
		MessagesPublished: 100,
		MessagesReceived:  80,
		Errors:            2,
		CPUTimeNanos:      1_000_000_000,
	})

	// Trigger a tick manually (called by the ticker goroutine in production)
	c.takeSnapshot()

	rate := c.GetRate("alice")
	if rate == nil {
		t.Fatal("GetRate(alice) returned nil")
	}
	if rate.MessagesPublished != 100 {
		t.Errorf("MessagesPublished = %d, want 100", rate.MessagesPublished)
	}
	if rate.MessagesReceived != 80 {
		t.Errorf("MessagesReceived = %d, want 80", rate.MessagesReceived)
	}
	if rate.Errors != 2 {
		t.Errorf("Errors = %d, want 2", rate.Errors)
	}
}

func TestCollector_AllRates(t *testing.T) {
	nc := startInProcessNATS(t)
	c := New(testLogger(), nc)

	c.SnapshotForTesting(context.Background(), "alice", Snapshot{MessagesPublished: 10})
	c.SnapshotForTesting(context.Background(), "bob", Snapshot{MessagesPublished: 20})
	c.takeSnapshot()

	all := c.AllRates()
	if len(all) != 2 {
		t.Fatalf("len(AllRates) = %d, want 2", len(all))
	}
	if all["alice"].MessagesPublished != 10 {
		t.Errorf("alice: %d, want 10", all["alice"].MessagesPublished)
	}
	if all["bob"].MessagesPublished != 20 {
		t.Errorf("bob: %d, want 20", all["bob"].MessagesPublished)
	}
}

func TestCollector_GetRate_Missing(t *testing.T) {
	nc := startInProcessNATS(t)
	c := New(testLogger(), nc)
	c.takeSnapshot()

	if rate := c.GetRate("nonexistent"); rate != nil {
		t.Errorf("GetRate(nonexistent) = %v, want nil", rate)
	}
}
