package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/quarkloop/supervisor/pkg/api"
)

// ListAgents returns every agent tracked by the supervisor.
func (c *Client) ListAgents(ctx context.Context) ([]api.AgentInfo, error) {
	var out []api.AgentInfo
	if err := c.do(ctx, http.MethodGet, c.route.Agents(), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// StartAgent launches an agent for the given space. workingDir is the
// directory the agent will chdir into (typically the user's cwd).
func (c *Client) StartAgent(ctx context.Context, space, workingDir string, port int) (api.AgentInfo, error) {
	var out api.AgentInfo
	err := c.do(ctx, http.MethodPost, c.route.Agents(), api.StartAgentRequest{
		Space:      space,
		WorkingDir: workingDir,
		Port:       port,
	}, &out)
	return out, err
}

// GetAgent returns the current state of a single agent by ID.
func (c *Client) GetAgent(ctx context.Context, id string) (api.AgentInfo, error) {
	var out api.AgentInfo
	err := c.do(ctx, http.MethodGet, c.route.Agent(id), nil, &out)
	return out, err
}

// AgentBySpace returns the running agent for the given space. Returns an
// HTTPError with status 404 when no agent is running for the space.
func (c *Client) AgentBySpace(ctx context.Context, space string) (api.AgentInfo, error) {
	agents, err := c.ListAgents(ctx)
	if err != nil {
		return api.AgentInfo{}, err
	}
	for _, a := range agents {
		if a.Space == space {
			return a, nil
		}
	}
	return api.AgentInfo{}, &HTTPError{
		Method:     http.MethodGet,
		Path:       c.route.Agents(),
		StatusCode: http.StatusNotFound,
		Body:       fmt.Sprintf("no agent running for space %q", space),
	}
}

// StopAgent requests termination of the agent by ID.
func (c *Client) StopAgent(ctx context.Context, id string) (api.AgentInfo, error) {
	var out api.AgentInfo
	err := c.do(ctx, http.MethodPost, c.route.AgentStop(id), nil, &out)
	return out, err
}
