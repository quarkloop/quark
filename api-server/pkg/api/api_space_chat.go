package api

import (
	"context"
	"fmt"

	"github.com/quarkloop/agent/pkg/agent"
)

// Chat sends a message to the supervisor agent of a running space and returns
// the reply. It POSTs to POST /api/v1/spaces/{id}/chat which the api-server
// proxies to the space-runtime's local /chat endpoint.
func (c *ClientApi) Chat(ctx context.Context, spaceID, message string) (*agent.ChatResponse, error) {
	var resp agent.ChatResponse
	path := fmt.Sprintf("/api/v1/spaces/%s/chat", spaceID)
	req := agent.ChatRequest{Message: message}
	if err := c.client.Post(ctx, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
