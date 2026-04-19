package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/quarkloop/supervisor/pkg/api"
)

// ListPlugins returns every installed plugin for a space. When typeFilter is
// non-empty, only plugins of that type are returned.
func (c *Client) ListPlugins(ctx context.Context, space, typeFilter string) ([]api.PluginInfo, error) {
	path := c.route.SpacePlugins(space)
	if typeFilter != "" {
		path += "?type=" + url.QueryEscape(typeFilter)
	}
	var resp api.ListPluginsResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Plugins, nil
}

// GetPlugin returns metadata for a single installed plugin.
func (c *Client) GetPlugin(ctx context.Context, space, plugin string) (api.PluginInfo, error) {
	var out api.PluginInfo
	err := c.do(ctx, http.MethodGet, c.route.SpacePlugin(space, plugin), nil, &out)
	return out, err
}

// InstallPlugin installs a plugin into the space from the given reference.
func (c *Client) InstallPlugin(ctx context.Context, space, ref string) (api.PluginInfo, error) {
	var resp api.InstallPluginResponse
	err := c.do(ctx, http.MethodPost, c.route.SpacePlugins(space),
		api.InstallPluginRequest{Ref: ref}, &resp)
	return resp.Plugin, err
}

// UninstallPlugin removes an installed plugin from the space.
func (c *Client) UninstallPlugin(ctx context.Context, space, plugin string) error {
	return c.do(ctx, http.MethodDelete, c.route.SpacePlugin(space, plugin), nil, nil)
}

// SearchPlugins returns hub search results for the given query.
func (c *Client) SearchPlugins(ctx context.Context, space, query string) ([]api.PluginSearchResult, error) {
	path := c.route.SpacePluginSearch(space) + "?q=" + url.QueryEscape(query)
	var resp api.SearchPluginsResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// HubPluginInfo returns detailed hub information for a plugin by name.
func (c *Client) HubPluginInfo(ctx context.Context, space, plugin string) (api.HubPluginInfo, error) {
	var out api.HubPluginInfo
	err := c.do(ctx, http.MethodGet, c.route.SpaceHubPlugin(space, plugin), nil, &out)
	return out, err
}
