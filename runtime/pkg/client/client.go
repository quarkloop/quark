package agentclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/quarkloop/supervisor/pkg/api"
)

type Client struct {
	transport *Transport
	basePath  string
}

type ClientOption func(*Client)

func New(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		transport: NewTransport(baseURL),
		basePath:  api.DefaultRuntimeBasePath,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
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

func (c *Client) Health(ctx context.Context) (*api.HealthResponse, error) {
	var resp api.HealthResponse
	if err := c.transport.Get(ctx, c.path(api.PathHealth), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Info(ctx context.Context) (*api.InfoResponse, error) {
	var resp api.InfoResponse
	if err := c.transport.Get(ctx, c.path(api.PathInfo), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Mode(ctx context.Context) (*api.ModeResponse, error) {
	var resp api.ModeResponse
	if err := c.transport.Get(ctx, c.path(api.PathMode), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) SetMode(ctx context.Context, mode string) (*api.ModeResponse, error) {
	var resp api.ModeResponse
	if err := c.transport.Post(ctx, c.path(api.PathMode), api.SetModeRequest{Mode: mode}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Stats(ctx context.Context) (*api.StatsResponse, error) {
	var resp api.StatsResponse
	if err := c.transport.Get(ctx, c.path(api.PathStats), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Chat(ctx context.Context, req api.AgentChatRequest) (*api.ChatResponse, error) {
	var resp api.ChatResponse
	if err := c.transport.Post(ctx, c.path(api.PathChat), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Stop(ctx context.Context) error {
	return c.transport.Post(ctx, c.path(api.PathStop), nil, nil)
}

func (c *Client) Plan(ctx context.Context) (*api.PlanResponse, error) {
	var resp api.PlanResponse
	if err := c.transport.Get(ctx, c.path(api.PathPlan), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Activity(ctx context.Context, limit int) ([]api.ActivityRecord, error) {
	path := "/v1/activity"
	if limit > 0 {
		path = fmt.Sprintf("%s?limit=%d", path, limit)
	}
	var resp []api.ActivityRecord
	if err := c.transport.Get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) ApprovePlan(ctx context.Context, planID string) (*api.PlanResponse, error) {
	var resp api.PlanResponse
	if err := c.transport.Post(ctx, c.path(api.PathPlanApprove), api.PlanActionRequest{PlanID: planID}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) RejectPlan(ctx context.Context, planID string) error {
	return c.transport.Post(ctx, c.path(api.PathPlanReject), api.PlanActionRequest{PlanID: planID}, nil)
}

func (c *Client) StreamActivity(ctx context.Context, fn func(api.ActivityRecord)) error {
	return c.transport.StreamSSEEvents(ctx, "/v1/activity/stream", func(event SSEEvent) error {
		var record api.ActivityRecord
		if err := json.Unmarshal(event.Data, &record); err != nil {
			fn(api.ActivityRecord{
				Type: "stream.decode_error",
				Data: mustRawJSON(map[string]string{"error": err.Error(), "chunk": string(event.Data)}),
			})
			return nil
		}
		fn(record)
		return nil
	})
}

func (c *Client) path(suffix string) string {
	return api.JoinPath(c.basePath, suffix)
}

func mustRawJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"error":"marshal failure"}`)
	}
	return data
}

func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}
