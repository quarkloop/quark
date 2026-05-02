// Package dag provides a directed acyclic graph for workflow step execution.
// It supports dependency-based ordering and parallel execution of independent steps.
package dag

import (
	"fmt"
	"sync"
	"time"
)

// StepStatus represents the execution status of a DAG step.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepReady     StepStatus = "ready" // dependencies satisfied, can run
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped" // skipped due to upstream failure
)

// DAGStep represents a single step in the DAG workflow.
type DAGStep struct {
	// ID is the unique identifier for this step.
	ID string

	// Name is a human-readable description.
	Name string

	// Action is the prompt or command to execute.
	Action string

	// DependsOn lists the IDs of steps that must complete before this step runs.
	DependsOn []string

	// Timeout is the maximum duration for this step.
	Timeout time.Duration

	// RetryCount is the number of times to retry on failure.
	RetryCount int

	// Status is the current execution status.
	Status StepStatus

	// Result is the output from step execution.
	Result string

	// Error is the error message if the step failed.
	Error string

	// Attempts is the number of execution attempts.
	Attempts int

	// StartedAt is when execution started.
	StartedAt time.Time

	// CompletedAt is when execution finished.
	CompletedAt time.Time
}

// DAG represents a directed acyclic graph of workflow steps.
type DAG struct {
	mu    sync.RWMutex
	steps map[string]*DAGStep
	order []string // topological order
}

// New creates a new DAG from the given steps.
func New(steps []DAGStep) (*DAG, error) {
	d := &DAG{
		steps: make(map[string]*DAGStep),
	}

	// Store pointers to slice elements directly (not copies)
	for i := range steps {
		steps[i].Status = StepPending
		d.steps[steps[i].ID] = &steps[i]
	}

	// Validate dependencies exist
	for _, s := range d.steps {
		for _, dep := range s.DependsOn {
			if _, ok := d.steps[dep]; !ok {
				return nil, fmt.Errorf("step %q depends on unknown step %q", s.ID, dep)
			}
		}
	}

	// Compute topological order
	order, err := d.topologicalSort()
	if err != nil {
		return nil, err
	}
	d.order = order

	return d, nil
}

// topologicalSort returns a topological ordering of steps using Kahn's algorithm.
func (d *DAG) topologicalSort() ([]string, error) {
	// Compute in-degree for each node
	inDegree := make(map[string]int)
	for id := range d.steps {
		inDegree[id] = 0
	}
	for _, s := range d.steps {
		for _, dep := range s.DependsOn {
			inDegree[s.ID]++ // s depends on dep, so s has higher in-degree
			_ = dep
		}
	}

	// Queue nodes with in-degree 0
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		// Pop from queue
		curr := queue[0]
		queue = queue[1:]
		order = append(order, curr)

		// Reduce in-degree for dependents
		for _, s := range d.steps {
			for _, dep := range s.DependsOn {
				if dep == curr {
					inDegree[s.ID]--
					if inDegree[s.ID] == 0 {
						queue = append(queue, s.ID)
					}
				}
			}
		}
	}

	if len(order) != len(d.steps) {
		return nil, fmt.Errorf("cycle detected in DAG")
	}
	return order, nil
}

// Get returns the step with the given ID.
func (d *DAG) Get(id string) (DAGStep, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	s, ok := d.steps[id]
	if !ok {
		return DAGStep{}, false
	}
	return *s, true
}

// Steps returns copies of all steps in topological order.
func (d *DAG) Steps() []DAGStep {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]DAGStep, 0, len(d.order))
	for _, id := range d.order {
		result = append(result, *d.steps[id])
	}
	return result
}

// Ready returns copies of all steps that are ready to execute.
// A step is ready if it has status StepReady (marked by UpdateReadySteps).
func (d *DAG) Ready() []DAGStep {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var ready []DAGStep
	for _, s := range d.steps {
		if s.Status == StepReady {
			ready = append(ready, *s)
		}
	}
	return ready
}

// dependenciesSatisfied checks if all dependencies of a step are completed.
// Caller must hold at least a read lock.
func (d *DAG) dependenciesSatisfied(s *DAGStep) bool {
	for _, dep := range s.DependsOn {
		depStep := d.steps[dep]
		if depStep == nil || depStep.Status != StepCompleted {
			return false
		}
	}
	return true
}

// anyDependencyFailed checks if any dependency has failed.
// Caller must hold at least a read lock.
func (d *DAG) anyDependencyFailed(s *DAGStep) bool {
	for _, dep := range s.DependsOn {
		depStep := d.steps[dep]
		if depStep != nil && (depStep.Status == StepFailed || depStep.Status == StepSkipped) {
			return true
		}
	}
	return false
}

// MarkReady marks a step as ready to run.
func (d *DAG) MarkReady(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	s := d.steps[id]
	if s == nil || s.Status != StepPending {
		return false
	}
	s.Status = StepReady
	return true
}

// MarkRunning marks a step as currently running.
func (d *DAG) MarkRunning(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	s := d.steps[id]
	if s == nil || (s.Status != StepPending && s.Status != StepReady) {
		return false
	}
	s.Status = StepRunning
	s.StartedAt = time.Now()
	s.Attempts++
	return true
}

// MarkCompleted marks a step as completed with a result.
func (d *DAG) MarkCompleted(id, result string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	s := d.steps[id]
	if s == nil || s.Status != StepRunning {
		return false
	}
	s.Status = StepCompleted
	s.Result = result
	s.CompletedAt = time.Now()
	return true
}

// MarkFailed marks a step as failed with an error.
func (d *DAG) MarkFailed(id, errMsg string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	s := d.steps[id]
	if s == nil || s.Status != StepRunning {
		return false
	}
	s.Status = StepFailed
	s.Error = errMsg
	s.CompletedAt = time.Now()

	// Skip downstream steps
	d.skipDownstream(id)
	return true
}

// ResetStep forces a failed step back to StepReady for retry.
// Clears the error and result so the step can be re-executed.
func (d *DAG) ResetStep(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	s := d.steps[id]
	if s == nil {
		return false
	}
	s.Status = StepReady
	s.Error = ""
	s.Result = ""
	s.CompletedAt = time.Time{}
	return true
}

// skipDownstream marks all steps that depend on the given step as skipped.
// Caller must hold the write lock.
func (d *DAG) skipDownstream(failedID string) {
	// Find all steps that directly or indirectly depend on failedID
	visited := make(map[string]bool)
	var skip func(id string)
	skip = func(id string) {
		for _, s := range d.steps {
			if visited[s.ID] {
				continue
			}
			for _, dep := range s.DependsOn {
				if dep == id {
					if s.Status == StepPending || s.Status == StepReady {
						s.Status = StepSkipped
						s.Error = fmt.Sprintf("skipped due to failure of %s", failedID)
						visited[s.ID] = true
						skip(s.ID) // recursively skip dependents
					}
				}
			}
		}
	}
	skip(failedID)
}

// CanRetry returns true if the step can be retried.
func (d *DAG) CanRetry(id string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	s := d.steps[id]
	if s == nil {
		return false
	}
	return s.Status == StepFailed && s.Attempts <= s.RetryCount
}

// IsComplete returns true if all steps have finished (completed, failed, or skipped).
func (d *DAG) IsComplete() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, s := range d.steps {
		switch s.Status {
		case StepPending, StepReady, StepRunning:
			return false
		}
	}
	return true
}

// HasFailures returns true if any step has failed.
func (d *DAG) HasFailures() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, s := range d.steps {
		if s.Status == StepFailed {
			return true
		}
	}
	return false
}

// Progress returns (completed, total) step counts.
func (d *DAG) Progress() (completed, total int) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, s := range d.steps {
		if s.Status == StepCompleted {
			completed++
		}
	}
	return completed, len(d.steps)
}

// UpdateReadySteps marks all steps whose dependencies are satisfied as ready.
// Returns the number of newly ready steps.
func (d *DAG) UpdateReadySteps() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	count := 0
	for _, s := range d.steps {
		if s.Status != StepPending {
			continue
		}
		if d.anyDependencyFailed(s) {
			s.Status = StepSkipped
			s.Error = "skipped due to upstream failure"
			continue
		}
		if d.dependenciesSatisfied(s) {
			s.Status = StepReady
			count++
		}
	}
	return count
}
