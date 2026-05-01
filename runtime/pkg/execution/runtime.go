package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/quarkloop/agent/pkg/approval"
	"github.com/quarkloop/agent/pkg/dag"
	"github.com/quarkloop/agent/pkg/loop"
)

// Runtime manages execution mode behavior at runtime.
type Runtime struct {
	config Config
	gate   *approval.Gate
	dag    *dag.DAG
}

// NewRuntime creates a new execution runtime with the given configuration.
func NewRuntime(config Config) (*Runtime, error) {
	r := &Runtime{config: config}

	switch config.Mode {
	case ModeAssistive:
		r.gate = approval.NewGate(config.ApprovalTimeout)

	case ModeWorkflow:
		if config.DAGConfig == nil {
			return nil, fmt.Errorf("workflow mode requires DAG configuration")
		}

		// Convert config to dag.Step slice
		steps := make([]dag.Step, len(config.DAGConfig.Steps))
		for i, s := range config.DAGConfig.Steps {
			steps[i] = dag.Step{
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
func (r *Runtime) Mode() Mode {
	return r.config.Mode
}

// Gate returns the approval gate for assistive mode.
// Returns nil if not in assistive mode.
func (r *Runtime) Gate() *approval.Gate {
	return r.gate
}

// DAG returns the workflow DAG for workflow mode.
// Returns nil if not in workflow mode.
func (r *Runtime) DAG() *dag.DAG {
	return r.dag
}

// ConfigureLoop applies execution mode configuration to the loop.
func (r *Runtime) ConfigureLoop(l *loop.Loop) {
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
func (r *Runtime) RunWorkflow(ctx context.Context, stepRunner dag.StepExecutor) error {
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
func (r *Runtime) Approve(id, reason string) bool {
	if r.gate == nil {
		return false
	}
	return r.gate.Approve(id, reason)
}

// Deny denies an approval request by ID. Only valid in assistive mode.
func (r *Runtime) Deny(id, reason string) bool {
	if r.gate == nil {
		return false
	}
	return r.gate.Deny(id, reason)
}

// PendingApprovals returns all pending approval requests. Only valid in assistive mode.
func (r *Runtime) PendingApprovals() []*approval.Request {
	if r.gate == nil {
		return nil
	}
	return r.gate.List()
}
