// Package plansvc defines the plan service interface and a local
// KB-backed implementation.
package plan

import (
	"context"

	agentapi "github.com/quarkloop/agent-api"
)

// Service defines operations for managing execution plans.
type Service interface {
	Create(ctx context.Context, goal string) (*agentapi.Plan, error)
	Get(ctx context.Context, planID string) (*agentapi.Plan, error)
	List(ctx context.Context) ([]agentapi.Plan, error)
	Approve(ctx context.Context, planID string) (*agentapi.Plan, error)
	Reject(ctx context.Context, planID string) error
	Update(ctx context.Context, planID string, status string) error
	Close() error
}
