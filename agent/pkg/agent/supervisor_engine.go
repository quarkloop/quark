package agent

import (
	"context"
	"log"
	"time"
)

// SupervisorEngine implements Engine for the default ReAct-style supervisor loop.
type SupervisorEngine struct{}

func NewSupervisorEngine() *SupervisorEngine {
	return &SupervisorEngine{}
}

func (s *SupervisorEngine) Run(ctx context.Context, exec *Executor) error {
	for {
		select {
		case <-ctx.Done():
			exec.saveCheckpoint()
			return ctx.Err()
		default:
		}
		done, err := exec.supervisorCycle(ctx)
		if err != nil {
			log.Printf("executor: supervisor cycle error: %v", err)
			select {
			case <-ctx.Done():
				exec.saveCheckpoint()
				return ctx.Err()
			case <-time.After(10 * time.Second):
			}
			continue
		}
		if done {
			log.Printf("executor: goal complete")
			exec.saveCheckpoint()
			return nil
		}
		exec.saveCheckpoint()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
