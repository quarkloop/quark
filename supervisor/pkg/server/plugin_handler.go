package server

import (
	"encoding/json"
	"net/http"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/pluginmanager"
)

// handleListPlugins serves GET /v1/spaces/{name}/plugins.
func (s *Server) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	mgr, err := s.store.Plugins(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}

	var plugins []pluginmanager.InstalledPlugin
	if typeFilter := r.URL.Query().Get("type"); typeFilter != "" {
		plugins, err = mgr.ListByType(plugin.PluginType(typeFilter))
	} else {
		plugins, err = mgr.List()
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := api.ListPluginsResponse{
		Plugins: make([]api.PluginInfo, 0, len(plugins)),
	}
	for _, p := range plugins {
		out.Plugins = append(out.Plugins, toAPIPluginInfo(p))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetPlugin serves GET /v1/spaces/{name}/plugins/{plugin}.
func (s *Server) handleGetPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pluginName := r.PathValue("plugin")

	mgr, err := s.store.Plugins(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	p, err := mgr.Get(pluginName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIPluginInfo(*p))
}

// handleInstallPlugin serves POST /v1/spaces/{name}/plugins.
func (s *Server) handleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	mgr, err := s.store.Plugins(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}

	var req api.InstallPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Ref == "" {
		writeError(w, http.StatusBadRequest, "ref is required")
		return
	}

	installed, err := mgr.Install(r.Context(), req.Ref)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, api.InstallPluginResponse{
		Plugin: toAPIPluginInfo(*installed),
	})
}

// handleUninstallPlugin serves DELETE /v1/spaces/{name}/plugins/{plugin}.
func (s *Server) handleUninstallPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pluginName := r.PathValue("plugin")

	mgr, err := s.store.Plugins(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	if err := mgr.Uninstall(pluginName); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSearchPlugins serves GET /v1/spaces/{name}/plugins/search?q=...
func (s *Server) handleSearchPlugins(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	mgr, err := s.store.Plugins(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}

	query := r.URL.Query().Get("q")
	results, err := mgr.Search(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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
	writeJSON(w, http.StatusOK, out)
}

// handleHubPluginInfo serves GET /v1/spaces/{name}/plugins/hub/{plugin}.
func (s *Server) handleHubPluginInfo(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pluginName := r.PathValue("plugin")

	mgr, err := s.store.Plugins(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	info, err := mgr.GetHubInfo(pluginName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, api.HubPluginInfo{
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
		Name:        p.Name,
		Version:     p.Version,
		Type:        string(p.Type),
		Mode:        string(p.Mode),
		Description: p.Description,
	}
}
