// Package configclient provides a unified config client that works in
// two modes: local (direct KB filesystem) or HTTP (remote agent API).
package config

import (
	"context"
	"fmt"

	agentclient "github.com/quarkloop/agent-client"
)

// Client is the unified config API.
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

// Get retrieves a config value by key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if c.local != nil {
		return c.local.Get(ctx, key)
	}
	switch key {
	case "mode":
		resp, err := c.remote.Mode(ctx)
		if err != nil {
			return "", err
		}
		return resp.Mode, nil
	default:
		return "", fmt.Errorf("config key %q is not available in HTTP mode", key)
	}
}

// Set stores a config value.
func (c *Client) Set(ctx context.Context, key string, value string) error {
	if c.local != nil {
		return c.local.Set(ctx, key, value)
	}
	switch key {
	case "mode":
		_, err := c.remote.SetMode(ctx, value)
		return err
	default:
		return fmt.Errorf("config key %q is not settable in HTTP mode", key)
	}
}

// Delete removes a config value.
func (c *Client) Delete(ctx context.Context, key string) error {
	if c.local != nil {
		return c.local.Delete(ctx, key)
	}
	switch key {
	case "mode":
		_, err := c.remote.SetMode(ctx, "auto")
		return err
	default:
		return fmt.Errorf("config key %q is not deletable in HTTP mode", key)
	}
}

// List returns all config values.
func (c *Client) List(ctx context.Context) (map[string]string, error) {
	if c.local != nil {
		return c.local.List(ctx)
	}
	out := make(map[string]string)
	info, err := c.remote.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("list config: %w", err)
	}
	out["agent_id"] = info.AgentID
	out["provider"] = info.Provider
	out["model"] = info.Model
	mode, err := c.remote.Mode(ctx)
	if err != nil {
		return nil, fmt.Errorf("list config: %w", err)
	}
	out["mode"] = mode.Mode
	return out, nil
}
