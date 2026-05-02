package dag

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// StepExecutor is the function type for running a single step.
// It receives the step action (prompt) and returns the result or error.
type StepExecutor func(ctx context.Context, stepID, action string) (result string, err error)

// ExecutorConfig configures the DAG runner.
type ExecutorConfig struct {
	// MaxParallel is the maximum number of steps to run in parallel. 0 = unlimited.
	MaxParallel int

	// DefaultTimeout is the default timeout for steps that don't specify one.
	DefaultTimeout time.Duration

	// OnStepStart is called when a step starts.
	OnStepStart func(step DAGStep)

	// OnStepComplete is called when a step completes (success or failure).
	OnStepComplete func(step DAGStep)
}

// Executor runs a DAG workflow with parallel step handling.
type Executor struct {
	dag    *DAG
	run    StepExecutor
	config ExecutorConfig

	mu      sync.Mutex
	running int
	sem     chan struct{} // semaphore for limiting parallelism
}

// NewExecutor creates a new DAG executor.
func NewExecutor(dag *DAG, runStep StepExecutor, config ExecutorConfig) *Executor {
	e := &Executor{
		dag:    dag,
		run:    runStep,
		config: config,
	}

	if config.MaxParallel > 0 {
		e.sem = make(chan struct{}, config.MaxParallel)
	}

	if config.DefaultTimeout == 0 {
		e.config.DefaultTimeout = 10 * time.Minute
	}

	return e
}

// Start runs the entire DAG, blocking until all steps complete or context is cancelled.
// Returns nil if all steps succeeded, or an error describing failures.
func (e *Executor) Start(ctx context.Context) error {
	// Initial pass to mark ready steps
	e.dag.UpdateReadySteps()

	var wg sync.WaitGroup
	errCh := make(chan error, len(e.dag.steps))

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		default:
		}

		// Check if we're done
		if e.dag.IsComplete() {
			break
		}

		// Get ready steps
		ready := e.dag.Ready()
		if len(ready) == 0 {
			// No steps ready but not complete — wait for running steps
			e.mu.Lock()
			running := e.running
			e.mu.Unlock()

			if running == 0 {
				// Deadlock or all remaining steps are blocked
				break
			}

			// Brief sleep to avoid busy-waiting
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// Launch ready steps
		for _, step := range ready {
			if !e.dag.MarkRunning(step.ID) {
				continue // already running or status changed
			}

			wg.Add(1)
			e.mu.Lock()
			e.running++
			e.mu.Unlock()

			go func(s DAGStep) {
				defer wg.Done()
				defer func() {
					e.mu.Lock()
					e.running--
					e.mu.Unlock()
				}()

				if err := e.runStep(ctx, s); err != nil {
					errCh <- fmt.Errorf("step %s failed: %w", s.ID, err)
				}

				// After step completes, update ready status of dependents
				e.dag.UpdateReadySteps()
			}(step)
		}

		// Brief sleep to avoid busy-loop
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for all running steps
	wg.Wait()
	close(errCh)

	// Collect errors
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d step(s) failed: %v", len(errors), errors[0])
	}

	if e.dag.HasFailures() {
		completed, total := e.dag.Progress()
		return fmt.Errorf("workflow incomplete: %d/%d steps completed", completed, total)
	}

	return nil
}

// runStep runs a single step with timeout and retry handling.
func (e *Executor) runStep(ctx context.Context, step DAGStep) error {
	// Acquire semaphore if limiting parallelism
	if e.sem != nil {
		select {
		case e.sem <- struct{}{}:
			defer func() { <-e.sem }()
		case <-ctx.Done():
			e.dag.MarkFailed(step.ID, ctx.Err().Error())
			return ctx.Err()
		}
	}

	// Notify start
	if e.config.OnStepStart != nil {
		e.config.OnStepStart(step)
	}

	// Determine timeout
	timeout := step.Timeout
	if timeout == 0 {
		timeout = e.config.DefaultTimeout
	}

	// Run with timeout
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := e.run(stepCtx, step.ID, step.Action)

	// Notify completion
	defer func() {
		if e.config.OnStepComplete != nil {
			s, ok := e.dag.Get(step.ID)
			if ok {
				e.config.OnStepComplete(s)
			}
		}
	}()

	if err != nil {
		// Check if we can retry
		if e.dag.CanRetry(step.ID) {
			slog.Warn("step failed, retrying",
				"step_id", step.ID, "attempt", step.Attempts, "retry_count", step.RetryCount+1, "error", err)

			// Reset status to allow retry
			e.dag.ResetStep(step.ID)

			return e.runStep(ctx, step)
		}

		e.dag.MarkFailed(step.ID, err.Error())
		return err
	}

	e.dag.MarkCompleted(step.ID, result)
	return nil
}

// Status returns a summary of the DAG progress.
type Status struct {
	Completed int
	Running   int
	Pending   int
	Failed    int
	Skipped   int
	Total     int
}

// GetStatus returns the current progress status.
func (e *Executor) GetStatus() Status {
	e.dag.mu.RLock()
	defer e.dag.mu.RUnlock()

	var s Status
	s.Total = len(e.dag.steps)
	for _, step := range e.dag.steps {
		switch step.Status {
		case StepCompleted:
			s.Completed++
		case StepRunning:
			s.Running++
		case StepPending, StepReady:
			s.Pending++
		case StepFailed:
			s.Failed++
		case StepSkipped:
			s.Skipped++
		}
	}
	return s
}
