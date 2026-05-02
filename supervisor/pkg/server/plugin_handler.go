package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/pluginmanager"
)

// handleListPlugins serves GET /v1/spaces/:name/plugins.
func (s *Server) handleListPlugins(c *fiber.Ctx) error {
	name := c.Params("name")
	mgr, err := s.store.Plugins(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}

	var plugins []pluginmanager.InstalledPlugin
	if typeFilter := c.Query("type"); typeFilter != "" {
		plugins, err = mgr.ListByType(plugin.PluginType(typeFilter))
	} else {
		plugins, err = mgr.List()
	}
	if err != nil {
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}

	out := api.ListPluginsResponse{
		Plugins: make([]api.PluginInfo, 0, len(plugins)),
	}
	for _, p := range plugins {
		out.Plugins = append(out.Plugins, toAPIPluginInfo(p))
	}
	return writeJSON(c, fiber.StatusOK, out)
}

// handleGetPlugin serves GET /v1/spaces/:name/plugins/:plugin.
func (s *Server) handleGetPlugin(c *fiber.Ctx) error {
	name := c.Params("name")
	pluginName := c.Params("plugin")

	mgr, err := s.store.Plugins(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	p, err := mgr.Get(pluginName)
	if err != nil {
		return writeError(c, fiber.StatusNotFound, err.Error())
	}
	return writeJSON(c, fiber.StatusOK, toAPIPluginInfo(p))
}

// handleInstallPlugin serves POST /v1/spaces/:name/plugins.
func (s *Server) handleInstallPlugin(c *fiber.Ctx) error {
	name := c.Params("name")
	mgr, err := s.store.Plugins(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}

	var req api.InstallPluginRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid request body: "+err.Error())
	}
	if req.Ref == "" {
		return writeError(c, fiber.StatusBadRequest, "ref is required")
	}

	installed, err := mgr.Install(c.Context(), req.Ref)
	if err != nil {
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}

	return writeJSON(c, fiber.StatusCreated, api.InstallPluginResponse{
		Plugin: toAPIPluginInfo(*installed),
	})
}

// handleUninstallPlugin serves DELETE /v1/spaces/:name/plugins/:plugin.
func (s *Server) handleUninstallPlugin(c *fiber.Ctx) error {
	name := c.Params("name")
	pluginName := c.Params("plugin")

	mgr, err := s.store.Plugins(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	if err := mgr.Uninstall(pluginName); err != nil {
		return writeError(c, fiber.StatusNotFound, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleSearchPlugins serves GET /v1/spaces/:name/plugins/search.
func (s *Server) handleSearchPlugins(c *fiber.Ctx) error {
	name := c.Params("name")
	mgr, err := s.store.Plugins(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}

	query := c.Query("q")
	results, err := mgr.Search(query)
	if err != nil {
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}

	out := api.SearchPluginsResponse{
		Results: make([]api.PluginSearchResult, 0, len(results)),
	}
	for _, res := range results {
		out.Results = append(out.Results, api.PluginSearchResult{
			Name:        res.Name,
			Version:     res.Version,
			Type:        res.Type,
			Description: res.Description,
			Author:      res.Author,
		})
	}
	return writeJSON(c, fiber.StatusOK, out)
}

// handleHubPluginInfo serves GET /v1/spaces/:name/plugins/hub/:plugin.
func (s *Server) handleHubPluginInfo(c *fiber.Ctx) error {
	name := c.Params("name")
	pluginName := c.Params("plugin")

	mgr, err := s.store.Plugins(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	info, err := mgr.GetHubInfo(pluginName)
	if err != nil {
		return writeError(c, fiber.StatusNotFound, err.Error())
	}

	return writeJSON(c, fiber.StatusOK, api.HubPluginInfo{
		Name:        info.Name,
		Version:     info.Version,
		Type:        info.Type,
		Description: info.Description,
		Author:      info.Author,
		License:     info.License,
		Repository:  info.Repository,
		Downloads:   info.Downloads,
		Versions:    info.Versions,
	})
}

func toAPIPluginInfo(p pluginmanager.InstalledPlugin) api.PluginInfo {
	return api.PluginInfo{
		Name:        p.Manifest.Name,
		Version:     p.Manifest.Version,
		Type:        string(p.Manifest.Type),
		Mode:        string(p.Manifest.Mode),
		Description: p.Manifest.Description,
	}
}
