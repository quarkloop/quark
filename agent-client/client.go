package agentclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	agentapi "github.com/quarkloop/agent-api"
)

type Client struct {
	transport *Transport
	basePath  string
}

type ClientOption func(*Client)

func New(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		transport: NewTransport(baseURL),
		basePath:  agentapi.DefaultBasePath,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func NewForAgent(apiServerURL, agentID string, opts ...ClientOption) *Client {
	opts = append([]ClientOption{WithBasePath(agentapi.AgentProxyBasePath(agentID))}, opts...)
	return New(apiServerURL, opts...)
}

func WithTransport(transport *Transport) ClientOption {
	return func(c *Client) {
		if transport != nil {
			c.transport = transport
		}
	}
}

func WithBasePath(basePath string) ClientOption {
	return func(c *Client) {
		if basePath != "" {
			c.basePath = basePath
		}
	}
}

func (c *Client) Transport() *Transport {
	return c.transport
}

func (c *Client) Health(ctx context.Context) (*agentapi.HealthResponse, error) {
	var resp agentapi.HealthResponse
	if err := c.transport.Get(ctx, c.path(agentapi.PathHealth), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Mode(ctx context.Context) (*agentapi.ModeResponse, error) {
	var resp agentapi.ModeResponse
	if err := c.transport.Get(ctx, c.path(agentapi.PathMode), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Stats(ctx context.Context) (agentapi.StatsResponse, error) {
	var resp agentapi.StatsResponse
	if err := c.transport.Get(ctx, c.path(agentapi.PathStats), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Chat(ctx context.Context, req agentapi.ChatRequest) (*agentapi.ChatResponse, error) {
	var resp agentapi.ChatResponse
	if err := c.transport.Post(ctx, c.path(agentapi.PathChat), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Stop(ctx context.Context) error {
	return c.transport.Post(ctx, c.path(agentapi.PathStop), nil, nil)
}

func (c *Client) Plan(ctx context.Context) (*agentapi.Plan, error) {
	var resp agentapi.Plan
	if err := c.transport.Get(ctx, c.path(agentapi.PathPlan), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Activity(ctx context.Context, limit int) ([]agentapi.ActivityRecord, error) {
	path := c.path(agentapi.PathActivity)
	if limit > 0 {
		path = fmt.Sprintf("%s?limit=%d", path, limit)
	}
	var resp []agentapi.ActivityRecord
	if err := c.transport.Get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) StreamActivity(ctx context.Context, fn func(agentapi.ActivityRecord)) error {
	return c.transport.StreamSSE(ctx, c.path(agentapi.PathActivityStream), func(chunk string) {
		var record agentapi.ActivityRecord
		if err := json.Unmarshal([]byte(chunk), &record); err != nil {
			fn(agentapi.ActivityRecord{
				Type: "stream.decode_error",
				Data: mustRawJSON(map[string]string{"error": err.Error(), "chunk": chunk}),
			})
			return
		}
		fn(record)
	})
}

func (c *Client) path(suffix string) string {
	return agentapi.JoinPath(c.basePath, suffix)
}

func mustRawJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"error":"marshal failure"}`)
	}
	return data
}

func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}
