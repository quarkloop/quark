package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/quarkloop/runtime/pkg/approval"
	"github.com/quarkloop/runtime/pkg/dag"
	"github.com/quarkloop/runtime/pkg/loop"
)

// ExecutionRuntime manages execution mode behavior at runtime.
type ExecutionRuntime struct {
	config Config
	gate   *approval.Gate
	dag    *dag.DAG
}

// NewExecutionRuntime creates a new execution runtime with the given configuration.
func NewExecutionRuntime(config Config) (*ExecutionRuntime, error) {
	r := &ExecutionRuntime{config: config}

	switch config.Mode {
	case ModeAssistive:
		r.gate = approval.NewGate(config.ApprovalTimeout)

	case ModeWorkflow:
		if config.DAGConfig == nil {
			return nil, fmt.Errorf("workflow mode requires DAG configuration")
		}

		// Convert config to dag.DAGStep slice
		steps := make([]dag.DAGStep, len(config.DAGConfig.Steps))
		for i, s := range config.DAGConfig.Steps {
			steps[i] = dag.DAGStep{
				ID:         s.ID,
				Name:       s.Name,
				Action:     s.Action,
				DependsOn:  s.DependsOn,
				Timeout:    s.Timeout,
				RetryCount: s.RetryCount,
			}
		}

		d, err := dag.New(steps)
		if err != nil {
			return nil, fmt.Errorf("invalid DAG: %w", err)
		}
		r.dag = d
	}

	return r, nil
}

// Mode returns the execution mode.
func (r *ExecutionRuntime) Mode() Mode {
	return r.config.Mode
}

// Gate returns the approval gate for assistive mode.
// Returns nil if not in assistive mode.
func (r *ExecutionRuntime) Gate() *approval.Gate {
	return r.gate
}

// DAG returns the workflow DAG for workflow mode.
// Returns nil if not in workflow mode.
func (r *ExecutionRuntime) DAG() *dag.DAG {
	return r.dag
}

// ConfigureLoop applies execution mode configuration to the loop.
func (r *ExecutionRuntime) ConfigureLoop(l *loop.Loop) {
	// Always add mode context middleware
	l.Use(ModeMiddleware(r.config.Mode))

	// Add mode-specific middleware
	switch r.config.Mode {
	case ModeAssistive:
		if r.gate != nil {
			l.Use(ApprovalMiddleware(r.gate))
		}
	}
}

// RunWorkflow starts the workflow DAG execution using the provided step executor.
// This should only be called in workflow mode.
func (r *ExecutionRuntime) RunWorkflow(ctx context.Context, stepRunner dag.StepExecutor) error {
	if r.config.Mode != ModeWorkflow {
		return fmt.Errorf("RunWorkflow called but mode is %s", r.config.Mode)
	}

	if r.dag == nil {
		return fmt.Errorf("no DAG configured")
	}

	config := dag.ExecutorConfig{
		MaxParallel:    r.config.DAGConfig.MaxParallel,
		DefaultTimeout: r.config.DAGConfig.DefaultTimeout,
	}

	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 10 * time.Minute
	}

	executor := dag.NewExecutor(r.dag, stepRunner, config)
	return executor.Start(ctx)
}

// Approve approves an approval request by ID. Only valid in assistive mode.
func (r *ExecutionRuntime) Approve(id, reason string) bool {
	if r.gate == nil {
		return false
	}
	return r.gate.Approve(id, reason)
}

// Deny denies an approval request by ID. Only valid in assistive mode.
func (r *ExecutionRuntime) Deny(id, reason string) bool {
	if r.gate == nil {
		return false
	}
	return r.gate.Deny(id, reason)
}

// PendingApprovals returns all pending approval requests. Only valid in assistive mode.
func (r *ExecutionRuntime) PendingApprovals() []*approval.Request {
	if r.gate == nil {
		return nil
	}
	return r.gate.List()
}
