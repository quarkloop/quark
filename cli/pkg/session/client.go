// Package sessionclient provides a unified session client that works in
// two modes: local (direct KB filesystem) or HTTP (remote agent API).
// The same API is used regardless of the transport.
package session

import (
	"context"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"
)

// Client is the unified session API.
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

// Create creates a new session.
func (c *Client) Create(ctx context.Context, req agentapi.CreateSessionRequest) (*agentapi.CreateSessionResponse, error) {
	if c.local != nil {
		return c.local.Create(ctx, req)
	}
	return c.remote.CreateSession(ctx, req)
}

// Get retrieves a session by key.
func (c *Client) Get(ctx context.Context, sessionKey string) (*agentapi.SessionRecord, error) {
	if c.local != nil {
		return c.local.Get(ctx, sessionKey)
	}
	return c.remote.Session(ctx, sessionKey)
}

// Delete removes a session by key.
func (c *Client) Delete(ctx context.Context, sessionKey string) error {
	if c.local != nil {
		return c.local.Delete(ctx, sessionKey)
	}
	return c.remote.DeleteSession(ctx, sessionKey)
}

// List returns all sessions.
func (c *Client) List(ctx context.Context) ([]agentapi.SessionRecord, error) {
	if c.local != nil {
		return c.local.List(ctx)
	}
	return c.remote.Sessions(ctx)
}
