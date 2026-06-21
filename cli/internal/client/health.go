package client

import (
	"context"
	"fmt"

	"github.com/quarkloop/quark/cli/internal/model"
)

// PlatformHealth returns the platform-wide health summary (admin — aggregates across all namespaces).
func (c *Client) PlatformHealth(ctx context.Context) (*model.HealthSummary, error) {
	var out model.HealthSummary
	if err := c.get(ctx, "/health", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// NamespaceHealth returns the health summary for a specific namespace.
func (c *Client) NamespaceHealth(ctx context.Context, namespace string) (*model.HealthSummary, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	var out model.HealthSummary
	path := fmt.Sprintf("/health/namespaces/%s", namespace)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SystemHealth returns the health breakdown for a specific system.
func (c *Client) SystemHealth(ctx context.Context, name, namespace string) (*model.SystemHealth, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	var out model.SystemHealth
	path := fmt.Sprintf("/health/systems/%s%s", name, buildQuery("namespace", namespace))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// NodeHealth returns the health of a specific node, including recent events.
func (c *Client) NodeHealth(ctx context.Context, name, namespace, system string) (*model.NodeHealth, error) {
	if namespace == "" || system == "" {
		return nil, fmt.Errorf("namespace and system are required")
	}
	var out model.NodeHealth
	path := fmt.Sprintf("/health/nodes/%s%s", name, buildQuery("namespace", namespace, "system", system))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
