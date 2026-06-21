package client

import (
	"context"
	"fmt"

	"github.com/quarkloop/quark/cli/internal/model"
)

// ListRegistry returns all registered node implementations.
// If category is non-empty, filters by category (e.g. "source", "function").
// If query is non-empty, performs a free-text search.
func (c *Client) ListRegistry(ctx context.Context, category, query string) ([]model.RegistryEntry, error) {
	var out []model.RegistryEntry
	path := "/registry" + buildQuery("category", category, "q", query)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetRegistryEntry looks up a specific implementation by URI.
// The uri parameter should be the full URI like "source/timer:v1".
func (c *Client) GetRegistryEntry(ctx context.Context, uri string) (*model.RegistryEntry, error) {
	var out model.RegistryEntry
	// The URI may contain slashes and colons; pass it as a path segment.
	path := fmt.Sprintf("/registry/%s", uri)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
