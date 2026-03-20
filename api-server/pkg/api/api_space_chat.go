package api

import (
	"context"
	"fmt"

	agentapi "github.com/quarkloop/agent-api"
)

// Chat sends a message to a running agent and returns the reply.
// It POSTs to POST /api/v1/agents/{id}/chat which the api-server
// proxies to the local agent runtime /chat endpoint.
func (c *ClientApi) Chat(ctx context.Context, agentID, message string) (*agentapi.ChatResponse, error) {
	var resp agentapi.ChatResponse
	path := fmt.Sprintf("/api/v1/agents/%s/chat", agentID)
	req := agentapi.ChatRequest{Message: message}
	if err := c.client.Post(ctx, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
