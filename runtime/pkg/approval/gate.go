package approval

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrDenied is returned when a tool call is denied by the human operator.
var ErrDenied = errors.New("approval denied")

// ErrExpired is returned when an approval request times out.
var ErrExpired = errors.New("approval request expired")

// Observer is notified when approval events occur.
type Observer interface {
	// OnRequestCreated is called when a new approval request is created.
	OnRequestCreated(req *Request)

	// OnRequestResolved is called when an approval request is resolved.
	OnRequestResolved(req *Request)
}

// Gate manages approval requests for tool calls in assistive mode.
// It blocks tool execution until human approval is received.
type Gate struct {
	mu       sync.RWMutex
	requests map[string]*Request
	waiters  map[string]chan Response
	timeout  time.Duration
	observer Observer
}

// NewGate creates a new approval gate with the specified timeout.
func NewGate(timeout time.Duration) *Gate {
	if timeout <= 0 {
		timeout = 24 * time.Hour // default
	}
	return &Gate{
		requests: make(map[string]*Request),
		waiters:  make(map[string]chan Response),
		timeout:  timeout,
	}
}

// SetObserver sets the observer for approval events.
func (g *Gate) SetObserver(obs Observer) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.observer = obs
}

// RequestApproval creates an approval request and blocks until it is resolved.
// Returns nil if approved, ErrDenied if denied, or ErrExpired if timed out.
func (g *Gate) RequestApproval(ctx context.Context, toolName, arguments, sessionID string) error {
	id := uuid.New().String()
	req := NewRequest(id, toolName, arguments, sessionID, g.timeout)

	// Create response channel
	respCh := make(chan Response, 1)

	g.mu.Lock()
	g.requests[id] = req
	g.waiters[id] = respCh
	obs := g.observer
	g.mu.Unlock()

	// Notify observer
	if obs != nil {
		obs.OnRequestCreated(req)
	}

	// Wait for resolution or timeout
	select {
	case resp := <-respCh:
		if resp.Approved {
			return nil
		}
		return ErrDenied
	case <-time.After(time.Until(req.ExpiresAt)):
		g.mu.Lock()
		req.Expire()
		delete(g.waiters, id)
		obs := g.observer
		g.mu.Unlock()
		if obs != nil {
			obs.OnRequestResolved(req)
		}
		return ErrExpired
	case <-ctx.Done():
		g.mu.Lock()
		delete(g.requests, id)
		delete(g.waiters, id)
		g.mu.Unlock()
		return ctx.Err()
	}
}

// Approve resolves an approval request with approval.
func (g *Gate) Approve(id, reason string) bool {
	g.mu.Lock()
	req, ok := g.requests[id]
	if !ok {
		g.mu.Unlock()
		return false
	}

	req.Approve(reason)
	ch := g.waiters[id]
	delete(g.waiters, id)
	obs := g.observer
	g.mu.Unlock()

	if ch != nil {
		ch <- Response{Approved: true, Reason: reason}
		close(ch)
	}

	if obs != nil {
		obs.OnRequestResolved(req)
	}
	return true
}

// Deny resolves an approval request with denial.
func (g *Gate) Deny(id, reason string) bool {
	g.mu.Lock()
	req, ok := g.requests[id]
	if !ok {
		g.mu.Unlock()
		return false
	}

	req.Deny(reason)
	ch := g.waiters[id]
	delete(g.waiters, id)
	obs := g.observer
	g.mu.Unlock()

	if ch != nil {
		ch <- Response{Approved: false, Reason: reason}
		close(ch)
	}

	if obs != nil {
		obs.OnRequestResolved(req)
	}
	return true
}

// Get returns the approval request with the given ID.
func (g *Gate) Get(id string) *Request {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.requests[id]
}

// List returns all pending approval requests.
func (g *Gate) List() []*Request {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var pending []*Request
	for _, req := range g.requests {
		if req.Status == StatusPending {
			// Check for expiry
			if !req.CheckExpiry() {
				pending = append(pending, req)
			}
		}
	}
	return pending
}

// Cleanup removes resolved requests older than the given duration.
func (g *Gate) Cleanup(maxAge time.Duration) int {
	g.mu.Lock()
	defer g.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	count := 0
	for id, req := range g.requests {
		if req.IsResolved() && req.ResolvedAt.Before(cutoff) {
			delete(g.requests, id)
			delete(g.waiters, id)
			count++
		}
	}
	return count
}
