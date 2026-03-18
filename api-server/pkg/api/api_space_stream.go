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

// StreamEvents streams events for a specific space ID.
func (c *ClientApi) StreamEvents(ctx context.Context, id string, fn func(string)) error {
	path := fmt.Sprintf("/api/v1/spaces/%s/events", id)
	return c.client.StreamSSE(ctx, path, fn)
}
