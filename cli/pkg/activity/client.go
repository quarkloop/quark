// Package activityclient provides a unified activity client that works in
// two modes: local (direct JSONL filesystem) or HTTP (remote agent API).
package activity

import (
	"context"
	"fmt"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"
)

// Client is the unified activity API.
// Exactly one of local or remote must be set.
type Client struct {
	local  Service
	remote *agentclient.Client
}

// NewLocal creates a client backed by the filesystem at spaceDir.
func NewLocal(spaceDir string) *Client {
	return &Client{local: NewLocalService(spaceDir)}
}

// NewHTTP creates a client that talks to a remote agent.
func NewHTTP(agentURL string) *Client {
	return &Client{remote: agentclient.New(agentURL)}
}

// Close releases resources.
func (c *Client) Close() error {
	if c.local != nil {
		return c.local.Close()
	}
	return nil
}

// Append appends an event to the activity log. Only supported in local mode.
func (c *Client) Append(ctx context.Context, record agentapi.ActivityRecord) error {
	if c.local != nil {
		return c.local.Append(ctx, record)
	}
	return fmt.Errorf("activity append is only supported in local mode")
}

// Query returns activity records matching the given options.
func (c *Client) Query(ctx context.Context, opts QueryOptions) ([]agentapi.ActivityRecord, error) {
	if c.local != nil {
		return c.local.Query(ctx, opts)
	}
	return c.remote.Activity(ctx, opts.Limit)
}

// Stream opens a live activity stream. Only supported in HTTP mode.
func (c *Client) Stream(ctx context.Context, fn func(agentapi.ActivityRecord)) error {
	if c.local != nil {
		return c.local.Stream(ctx, fn)
	}
	return c.remote.StreamActivity(ctx, fn)
}
