// Package subagent provides bounded sub-agent execution with explicit
// resource limits: depth, concurrency, token budget, and wall-clock timeout.
// Subagents communicate results back to the parent via EventBus.
package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/plan"
)

// Config defines resource boundaries for a subagent.
type Config struct {
	MaxDepth      int           // max recursion depth (default: 3)
	MaxConcurrent int           // max simultaneous subagents (default: 5)
	TokenBudget   int           // tokens for this subagent's context window
	Timeout       time.Duration // wall-clock timeout (default: 5 min)
	MaxMessages   int           // ephemeral context message limit (default: 50)
}

// Status tracks a subagent's lifecycle state.
type Status string

const (
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusOrphaned  Status = "orphaned"
	StatusTimedOut  Status = "timed_out"
)

// Result is the output of a completed subagent.
type Result struct {
	StepID string
	Output string
	Error  error
}

// Subagent tracks a running sub-agent instance.
type Subagent struct {
	ID        string
	ParentID  string
	Depth     int
	Config    Config
	SessionID string
	Cancel    context.CancelFunc
	StartedAt time.Time
	Status    Status
}

// Manager tracks all active subagents and enforces resource limits.
type Manager struct {
	mu        sync.Mutex
	active    map[string]*Subagent
	semaphore chan struct{}
	bus       *eventbus.Bus
}

// NewManager creates a Manager with the given concurrency limit.
func NewManager(maxConcurrent int, bus *eventbus.Bus) *Manager {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	return &Manager{
		active:    make(map[string]*Subagent),
		semaphore: make(chan struct{}, maxConcurrent),
		bus:       bus,
	}
}

// Spawn creates and starts a subagent. Blocks if at concurrency limit.
// Returns error if depth limit exceeded.
func (m *Manager) Spawn(ctx context.Context, parentID string, depth int, cfg Config, step plan.Step, runFunc func(ctx context.Context, step plan.Step, tokenBudget int, maxMessages int) (string, error)) (*Subagent, error) {
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	if cfg.MaxMessages <= 0 {
		cfg.MaxMessages = 50
	}
	if cfg.TokenBudget <= 0 {
		cfg.TokenBudget = 4096
	}

	if depth >= cfg.MaxDepth {
		return nil, fmt.Errorf("subagent depth limit %d reached", cfg.MaxDepth)
	}

	// Acquire semaphore (blocks if at limit).
	select {
	case m.semaphore <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	sub := &Subagent{
		ID:        step.ID,
		ParentID:  parentID,
		Depth:     depth,
		Config:    cfg,
		SessionID: fmt.Sprintf("agent:%s:subagent:%s", parentID, step.ID),
		StartedAt: time.Now().UTC(),
		Status:    StatusRunning,
	}

	subCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	sub.Cancel = cancel

	m.mu.Lock()
	m.active[sub.ID] = sub
	m.mu.Unlock()

	go func() {
		defer cancel()
		defer func() { <-m.semaphore }()

		output, err := runFunc(subCtx, step, cfg.TokenBudget, cfg.MaxMessages)

		m.mu.Lock()
		if err != nil {
			sub.Status = StatusFailed
			if subCtx.Err() == context.DeadlineExceeded {
				sub.Status = StatusTimedOut
			}
		} else {
			sub.Status = StatusCompleted
			_ = output
		}
		delete(m.active, sub.ID)
		m.mu.Unlock()

		// Deliver result via EventBus.
		if m.bus != nil {
			data := map[string]string{
				"step":  step.ID,
				"agent": sub.ID,
			}
			if err != nil {
				data["error"] = err.Error()
				m.bus.Emit(eventbus.Event{
					Kind:      eventbus.KindStepFailed,
					SessionID: sub.SessionID,
					Data:      data,
				})
			} else {
				m.bus.Emit(eventbus.Event{
					Kind:      eventbus.KindStepCompleted,
					SessionID: sub.SessionID,
					Data:      data,
				})
			}
		}
	}()

	return sub, nil
}

// Wait blocks until the subagent completes or the context is cancelled.
func (m *Manager) Wait(ctx context.Context, id string) (*Result, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			m.mu.Lock()
			_, exists := m.active[id]
			m.mu.Unlock()
			if !exists {
				return &Result{StepID: id}, nil
			}
		}
	}
}

// Cancel terminates a specific subagent.
func (m *Manager) Cancel(id string) {
	m.mu.Lock()
	sub, ok := m.active[id]
	if ok {
		sub.Status = StatusFailed
		sub.Cancel()
		delete(m.active, id)
	}
	m.mu.Unlock()
}

// CancelAll terminates all subagents (parent shutdown).
func (m *Manager) CancelAll() {
	m.mu.Lock()
	for id, sub := range m.active {
		sub.Status = StatusOrphaned
		sub.Cancel()
		delete(m.active, id)
	}
	m.mu.Unlock()
}

// DetectOrphans checks for subagents whose parent is no longer alive.
func (m *Manager) DetectOrphans(aliveParents map[string]bool) {
	m.mu.Lock()
	for id, sub := range m.active {
		if !aliveParents[sub.ParentID] {
			sub.Status = StatusOrphaned
			sub.Cancel()
			delete(m.active, id)
		}
	}
	m.mu.Unlock()
}

// Active returns the number of running subagents.
func (m *Manager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.active)
}
