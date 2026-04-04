// Package planclient provides a unified plan client that works in
// two modes: local (direct KB filesystem) or HTTP (remote agent API).
package plan

import (
	"context"
	"fmt"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"
)

// Client is the unified plan API.
// Exactly one of local or remote must be set.
type Client struct {
	local  Service
	remote *agentclient.Client
}

// NewLocal creates a client backed by the filesystem at spaceDir.
func NewLocal(spaceDir string) (*Client, error) {
	svc, err := NewLocalService(spaceDir)
	if err != nil {
		return nil, err
	}
	return &Client{local: svc}, nil
}

// NewHTTP creates a client that talks to a remote agent.
func NewHTTP(agentURL string) *Client {
	return &Client{remote: agentclient.New(agentURL)}
}

// Close releases resources. Only meaningful for local clients.
func (c *Client) Close() error {
	if c.local != nil {
		return c.local.Close()
	}
	return nil
}

// Create creates a new draft plan. Only supported in local mode.
func (c *Client) Create(ctx context.Context, goal string) (*agentapi.Plan, error) {
	if c.local != nil {
		return c.local.Create(ctx, goal)
	}
	return nil, fmt.Errorf("plan create is only supported in local mode")
}

// Get retrieves a plan. In HTTP mode, returns the agent's current plan.
func (c *Client) Get(ctx context.Context, planID string) (*agentapi.Plan, error) {
	if c.local != nil {
		return c.local.Get(ctx, planID)
	}
	return c.remote.Plan(ctx)
}

// List returns all plans. In HTTP mode, returns the agent's current plan.
func (c *Client) List(ctx context.Context) ([]agentapi.Plan, error) {
	if c.local != nil {
		return c.local.List(ctx)
	}
	p, err := c.remote.Plan(ctx)
	if err != nil {
		return nil, err
	}
	return []agentapi.Plan{*p}, nil
}

// Approve approves a plan. In HTTP mode, planID is ignored (agent has one current plan).
func (c *Client) Approve(ctx context.Context, planID string) (*agentapi.Plan, error) {
	if c.local != nil {
		return c.local.Approve(ctx, planID)
	}
	return c.remote.ApprovePlan(ctx, planID)
}

// Reject rejects a plan. In HTTP mode, planID is ignored.
func (c *Client) Reject(ctx context.Context, planID string) error {
	if c.local != nil {
		return c.local.Reject(ctx, planID)
	}
	return c.remote.RejectPlan(ctx, planID)
}

// Update updates a plan's status. Only supported in local mode.
func (c *Client) Update(ctx context.Context, planID string, status string) error {
	if c.local != nil {
		return c.local.Update(ctx, planID, status)
	}
	return fmt.Errorf("plan update is only supported in local mode")
}
