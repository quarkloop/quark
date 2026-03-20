package api

import (
	"context"
	"fmt"
)

// StreamLogs streams logs for a specific space ID.
func (c *ClientApi) StreamLogs(ctx context.Context, id string, fn func(string)) error {
	path := fmt.Sprintf("/api/v1/spaces/%s/logs", id)
	return c.client.StreamSSE(ctx, path, fn)
}

// StreamActivity streams activity for a specific agent ID.
func (c *ClientApi) StreamActivity(ctx context.Context, id string, fn func(string)) error {
	path := fmt.Sprintf("/api/v1/agents/%s/activity/stream", id)
	return c.client.StreamSSE(ctx, path, fn)
}
