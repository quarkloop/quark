package client

import (
	"context"
	"net/http"

	"github.com/quarkloop/supervisor/pkg/api"
)

// KBGet returns the value stored at namespace/key in the given space.
func (c *Client) KBGet(ctx context.Context, space, namespace, key string) ([]byte, error) {
	var resp api.KBValueResponse
	if err := c.do(ctx, http.MethodGet, c.route.SpaceKBItem(space, namespace, key), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Value, nil
}

// KBSet stores value at namespace/key in the given space.
func (c *Client) KBSet(ctx context.Context, space, namespace, key string, value []byte) error {
	return c.do(ctx, http.MethodPut, c.route.SpaceKBItem(space, namespace, key),
		api.KBSetRequest{Value: value}, nil)
}

// KBDelete removes the entry at namespace/key in the given space.
func (c *Client) KBDelete(ctx context.Context, space, namespace, key string) error {
	return c.do(ctx, http.MethodDelete, c.route.SpaceKBItem(space, namespace, key), nil, nil)
}

// KBList returns the keys in a namespace.
func (c *Client) KBList(ctx context.Context, space, namespace string) ([]string, error) {
	var resp api.KBListResponse
	if err := c.do(ctx, http.MethodGet, c.route.SpaceKBList(space, namespace), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Keys, nil
}
