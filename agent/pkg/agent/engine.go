package agent

import (
	"context"
)

// Engine defines the execution strategy for an agent space.
// It is responsible for orchestrating the overall goal lifecycle.
// The Executor provides access to all necessary resources (KB, Gateway, etc).
type Engine interface {
	// Run executes the agent strategy, blocking until the goal completes
	// or the context is cancelled.
	Run(ctx context.Context, exec *Executor) error
}
