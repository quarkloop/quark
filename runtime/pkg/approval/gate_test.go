package approval_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/quarkloop/runtime/pkg/approval"
)

func TestGateApprove(t *testing.T) {
	gate := approval.NewGate(5 * time.Second)

	var wg sync.WaitGroup
	wg.Add(1)

	var approvalErr error
	go func() {
		defer wg.Done()
		approvalErr = gate.RequestApproval(context.Background(), "bash", `{"cmd":"ls"}`, "session-1")
	}()

	// Wait a bit for the request to be created
	time.Sleep(50 * time.Millisecond)

	// Check that request exists
	pending := gate.List()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(pending))
	}

	// Approve it
	if !gate.Approve(pending[0].ID, "test approval") {
		t.Fatal("approve returned false")
	}

	wg.Wait()

	if approvalErr != nil {
		t.Errorf("expected nil error after approval, got: %v", approvalErr)
	}
}

func TestGateDeny(t *testing.T) {
	gate := approval.NewGate(5 * time.Second)

	var wg sync.WaitGroup
	wg.Add(1)

	var approvalErr error
	go func() {
		defer wg.Done()
		approvalErr = gate.RequestApproval(context.Background(), "bash", `{"cmd":"rm -rf"}`, "session-1")
	}()

	// Wait a bit for the request to be created
	time.Sleep(50 * time.Millisecond)

	pending := gate.List()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(pending))
	}

	// Deny it
	if !gate.Deny(pending[0].ID, "dangerous command") {
		t.Fatal("deny returned false")
	}

	wg.Wait()

	if approvalErr != approval.ErrDenied {
		t.Errorf("expected ErrDenied, got: %v", approvalErr)
	}
}

func TestGateExpiry(t *testing.T) {
	gate := approval.NewGate(100 * time.Millisecond)

	err := gate.RequestApproval(context.Background(), "bash", `{}`, "session-1")

	if err != approval.ErrExpired {
		t.Errorf("expected ErrExpired, got: %v", err)
	}
}

func TestGateContextCancellation(t *testing.T) {
	gate := approval.NewGate(10 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := gate.RequestApproval(ctx, "bash", `{}`, "session-1")

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}

type testObserver struct {
	mu       sync.Mutex
	created  int
	resolved int
}

func (o *testObserver) OnRequestCreated(_ *approval.Request) {
	o.mu.Lock()
	o.created++
	o.mu.Unlock()
}

func (o *testObserver) OnRequestResolved(_ *approval.Request) {
	o.mu.Lock()
	o.resolved++
	o.mu.Unlock()
}

func TestGateObserver(t *testing.T) {
	gate := approval.NewGate(5 * time.Second)
	obs := &testObserver{}
	gate.SetObserver(obs)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		_ = gate.RequestApproval(context.Background(), "bash", `{}`, "session-1")
	}()

	time.Sleep(50 * time.Millisecond)

	pending := gate.List()
	gate.Approve(pending[0].ID, "ok")

	wg.Wait()

	obs.mu.Lock()
	if obs.created != 1 {
		t.Errorf("expected 1 created event, got %d", obs.created)
	}
	if obs.resolved != 1 {
		t.Errorf("expected 1 resolved event, got %d", obs.resolved)
	}
	obs.mu.Unlock()
}
