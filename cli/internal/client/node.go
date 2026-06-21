package client

import (
	"context"
	"fmt"

	"github.com/quarkloop/quark/cli/internal/model"
)

// ListNodes returns all nodes in a namespace (optionally within a single system).
func (c *Client) ListNodes(ctx context.Context, namespace, system string) ([]model.NodeSummary, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	var out []model.NodeSummary
	path := "/nodes" + buildQuery("namespace", namespace, "system", system)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetNode returns detailed information about a single node.
func (c *Client) GetNode(ctx context.Context, name, namespace, system string) (*model.NodeDetail, error) {
	if namespace == "" || system == "" {
		return nil, fmt.Errorf("namespace and system are required")
	}
	var out model.NodeDetail
	path := fmt.Sprintf("/nodes/%s%s", name, buildQuery("namespace", namespace, "system", system))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// NodeLifecycle performs a lifecycle operation (pause/resume/drain/archive/recover/delete)
// on a node. The action parameter must be one of: "pause", "resume", "drain",
// "archive", "recover", "delete".
func (c *Client) NodeLifecycle(ctx context.Context, action, name, namespace, system string) error {
	if namespace == "" || system == "" {
		return fmt.Errorf("namespace and system are required")
	}
	path := fmt.Sprintf("/nodes/%s/%s%s", name, action, buildQuery("namespace", namespace, "system", system))
	return c.post(ctx, path, nil, nil)
}
