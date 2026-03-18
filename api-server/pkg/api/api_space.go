package api

import (
	"context"
	"fmt"

	"github.com/quarkloop/api-server/pkg/space"
)

// RunSpace calls POST /api/v1/spaces to create and launch a new space.
// env values are forwarded as OS environment variables to the space-runtime.
// restartPolicy must be "on-failure", "always", or "never".
func (c *ClientApi) RunSpace(ctx context.Context, name, dir string, env map[string]string, restartPolicy string) (*space.Space, error) {
	var sp space.Space
	req := space.RunRequest{Name: name, Dir: dir, Env: env, RestartPolicy: restartPolicy}
	err := c.client.Post(ctx, "/api/v1/spaces", req, &sp)
	return &sp, err
}

// ListSpaces calls GET /api/v1/spaces and returns all records in creation order.
func (c *ClientApi) ListSpaces(ctx context.Context) ([]*space.Space, error) {
	var spaces []*space.Space
	err := c.client.Get(ctx, "/api/v1/spaces", &spaces)
	return spaces, err
}

// GetSpace calls GET /api/v1/spaces/{id} and returns the current record.
func (c *ClientApi) GetSpace(ctx context.Context, id string) (*space.Space, error) {
	var sp space.Space
	path := fmt.Sprintf("/api/v1/spaces/%s", id)
	err := c.client.Get(ctx, path, &sp)
	return &sp, err
}

// StopSpace calls POST /api/v1/spaces/{id}/stop.
// force=true sends SIGKILL; false sends SIGINT.
func (c *ClientApi) StopSpace(ctx context.Context, id string, force bool) error {
	path := fmt.Sprintf("/api/v1/spaces/%s/stop", id)
	req := space.StopRequest{Force: force}
	return c.client.Post(ctx, path, req, nil)
}

// DeleteSpace calls DELETE /api/v1/spaces/{id}.
// The space must already be stopped or failed; the server returns 409 otherwise.
func (c *ClientApi) DeleteSpace(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/spaces/%s", id)
	return c.client.Delete(ctx, path, nil, nil)
}

// PruneSpaces removes all stopped and failed space records.
// Returns the list of IDs that were pruned.
func (c *ClientApi) PruneSpaces(ctx context.Context) ([]string, error) {
	var result struct {
		Pruned []string `json:"pruned"`
	}
	if err := c.client.Post(ctx, "/api/v1/spaces/prune", nil, &result); err != nil {
		return nil, err
	}
	return result.Pruned, nil
}

// GetSpaceStats calls GET /api/v1/spaces/{id}/stats, which the api-server
// proxies to the space-runtime's local /stats endpoint.
func (c *ClientApi) GetSpaceStats(ctx context.Context, id string) (map[string]interface{}, error) {
	var stats map[string]interface{}
	path := fmt.Sprintf("/api/v1/spaces/%s/stats", id)
	err := c.client.Get(ctx, path, &stats)
	return stats, err
}
