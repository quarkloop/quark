package hierarchy

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/quarkloop/runtime/pkg/loop"
)

// WorkItem represents a unit of work that can be delegated to a sub-agent.
type WorkItem struct {
	loop.BaseMessage
	AgentID    string // Target agent ID
	Task       string // Task description or prompt
	Timeout    time.Duration
	ResultChan chan WorkResult
}

// WorkResult holds the result of delegated work.
type WorkResult struct {
	AgentID  string
	Success  bool
	Result   string
	Error    string
	Duration time.Duration
}

// Type returns the message type for loop dispatch.
func (w WorkItem) Type() string {
	return "work_item"
}

// Delegator manages work delegation between agents.
type Delegator struct {
	mu       sync.RWMutex
	registry *Registry
	loops    map[string]*loop.Loop // agent ID -> loop
	pending  map[string][]WorkItem // agent ID -> pending work
}

// NewDelegator creates a new work delegator.
func NewDelegator(registry *Registry) *Delegator {
	return &Delegator{
		registry: registry,
		loops:    make(map[string]*loop.Loop),
		pending:  make(map[string][]WorkItem),
	}
}

// RegisterLoop associates a loop with an agent.
func (d *Delegator) RegisterLoop(agentID string, l *loop.Loop) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.loops[agentID] = l
}

// UnregisterLoop removes the loop association for an agent.
func (d *Delegator) UnregisterLoop(agentID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.loops, agentID)
}

// Delegate sends a work item to a sub-agent's loop.
// Returns a channel that will receive the result.
func (d *Delegator) Delegate(ctx context.Context, parentID string, work WorkItem) (chan WorkResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Validate parent exists
	_, ok := d.registry.Get(parentID)
	if !ok {
		return nil, errors.New("parent agent not found")
	}

	// Validate target agent exists and is a child
	target, ok := d.registry.Get(work.AgentID)
	if !ok {
		return nil, fmt.Errorf("target agent %s not found", work.AgentID)
	}
	if target.Identity.ParentID != parentID {
		return nil, fmt.Errorf("agent %s is not a child of %s", work.AgentID, parentID)
	}

	// Get target's loop
	targetLoop, ok := d.loops[work.AgentID]
	if !ok {
		// Queue for later if loop not ready
		work.ResultChan = make(chan WorkResult, 1)
		d.pending[work.AgentID] = append(d.pending[work.AgentID], work)
		return work.ResultChan, nil
	}

	// Create result channel if not provided
	if work.ResultChan == nil {
		work.ResultChan = make(chan WorkResult, 1)
	}

	// Send to target loop
	targetLoop.Send(work)

	return work.ResultChan, nil
}

// DelegateAndWait sends work and blocks until completion or timeout.
func (d *Delegator) DelegateAndWait(ctx context.Context, parentID string, work WorkItem) (WorkResult, error) {
	resultChan, err := d.Delegate(ctx, parentID, work)
	if err != nil {
		return WorkResult{}, err
	}

	timeout := work.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	select {
	case result := <-resultChan:
		return result, nil
	case <-time.After(timeout):
		return WorkResult{
			AgentID: work.AgentID,
			Success: false,
			Error:   "delegation timed out",
		}, nil
	case <-ctx.Done():
		return WorkResult{
			AgentID: work.AgentID,
			Success: false,
			Error:   ctx.Err().Error(),
		}, ctx.Err()
	}
}

// FlushPending sends any pending work items to a newly registered loop.
func (d *Delegator) FlushPending(agentID string) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	pending := d.pending[agentID]
	if len(pending) == 0 {
		return 0
	}

	l, ok := d.loops[agentID]
	if !ok {
		return 0
	}

	for _, work := range pending {
		l.Send(work)
	}

	count := len(pending)
	delete(d.pending, agentID)
	return count
}

// PendingCount returns the number of pending work items for an agent.
func (d *Delegator) PendingCount(agentID string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.pending[agentID])
}

// WorkHandler creates a loop handler for processing work items.
func WorkHandler(registry *Registry, processor WorkProcessor) loop.HandlerFunc {
	return func(ctx context.Context, msg loop.Message) error {
		work, ok := msg.(WorkItem)
		if !ok {
			return fmt.Errorf("expected WorkItem, got %T", msg)
		}

		start := time.Now()

		// Update status
		registry.SetStatus(work.AgentID, StatusRunning)

		// Process the work
		result, err := processor(ctx, work.AgentID, work.Task)

		duration := time.Since(start)

		if err != nil {
			registry.SetStatus(work.AgentID, StatusFailed)
			registry.SetError(work.AgentID, err.Error())

			if work.ResultChan != nil {
				work.ResultChan <- WorkResult{
					AgentID:  work.AgentID,
					Success:  false,
					Error:    err.Error(),
					Duration: duration,
				}
			}
			return err
		}

		registry.SetStatus(work.AgentID, StatusComplete)
		registry.SetResult(work.AgentID, result)

		if work.ResultChan != nil {
			work.ResultChan <- WorkResult{
				AgentID:  work.AgentID,
				Success:  true,
				Result:   result,
				Duration: duration,
			}
		}

		return nil
	}
}

// WorkProcessor is a function that processes a work item and returns a result.
type WorkProcessor func(ctx context.Context, agentID, task string) (string, error)
