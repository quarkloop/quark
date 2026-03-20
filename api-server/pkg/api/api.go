// Package api provides ClientApi, the typed Go client used by CLI commands
// to talk to the api-server over HTTP.
//
// Each method maps 1-to-1 to a REST endpoint. Low-level details (JSON
// encode/decode, error parsing, SSE streaming) are handled by client.Client.
//
// Example:
//
//	c := api.NewClientApi("http://127.0.0.1:7070")
//	sp, _ := c.RunSpace(ctx, name, dir, env, "on-failure")
package api

import (
	agentclient "github.com/quarkloop/agent-client"
	"github.com/quarkloop/api-server/pkg/api/client"
)

// ClientApi is a typed HTTP client for the quark api-server.
// The zero value is not usable; construct with NewClientApi.
// ClientApi is the typed HTTP client used by all CLI commands to communicate
// with the quark api-server. Obtain one via NewClientApi(apiServerURL()).
// The zero value is not usable.
type ClientApi struct {
	client *client.Client
}

// NewClientApi returns a ClientApi targeting serverURL (e.g. "http://127.0.0.1:7070").
// NewClientApi returns a ClientApi that sends requests to serverURL
// (e.g. "http://127.0.0.1:7070").
func NewClientApi(serverURL string) *ClientApi {
	return &ClientApi{
		client: client.NewClient(serverURL),
	}
}

func (c *ClientApi) Agent(agentID string) *agentclient.Client {
	return agentclient.NewForAgent(c.client.BaseURL(), agentID, agentclient.WithTransport(c.client))
}
