package client

import (
	"context"
	"net/http"

	"github.com/quarkloop/supervisor/pkg/api"
)

// ListSpaces returns every space known to the supervisor.
func (c *Client) ListSpaces(ctx context.Context) ([]api.SpaceInfo, error) {
	var out []api.SpaceInfo
	if err := c.do(ctx, http.MethodGet, c.route.Spaces(), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateSpace registers a new space. quarkfile is the raw bytes of the user's
// Quarkfile; meta.name inside must match name. The supervisor writes the
// Quarkfile to workingDir.
func (c *Client) CreateSpace(ctx context.Context, name string, quarkfile []byte, workingDir string) (api.SpaceInfo, error) {
	var out api.SpaceInfo
	err := c.do(ctx, http.MethodPost, c.route.Spaces(), api.CreateSpaceRequest{
		Name:       name,
		Quarkfile:  quarkfile,
		WorkingDir: workingDir,
	}, &out)
	return out, err
}

// GetSpace returns metadata for a single space.
func (c *Client) GetSpace(ctx context.Context, name string) (api.SpaceInfo, error) {
	var out api.SpaceInfo
	err := c.do(ctx, http.MethodGet, c.route.Space(name), nil, &out)
	return out, err
}

// DeleteSpace permanently removes a space and all of its data.
func (c *Client) DeleteSpace(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, c.route.Space(name), nil, nil)
}

// Quarkfile returns the latest stored Quarkfile for the space.
func (c *Client) Quarkfile(ctx context.Context, name string) (api.QuarkfileResponse, error) {
	var out api.QuarkfileResponse
	err := c.do(ctx, http.MethodGet, c.route.SpaceQuarkfile(name), nil, &out)
	return out, err
}

// UpdateQuarkfile replaces the latest Quarkfile for the space.
func (c *Client) UpdateQuarkfile(ctx context.Context, name string, quarkfile []byte) (api.SpaceInfo, error) {
	var out api.SpaceInfo
	err := c.do(ctx, http.MethodPut, c.route.SpaceQuarkfile(name),
		api.UpdateQuarkfileRequest{Quarkfile: quarkfile}, &out)
	return out, err
}

// Doctor runs supervisor-side health checks against the space.
func (c *Client) Doctor(ctx context.Context, name string) (api.DoctorResponse, error) {
	var out api.DoctorResponse
	err := c.do(ctx, http.MethodPost, c.route.SpaceDoctor(name), nil, &out)
	return out, err
}
