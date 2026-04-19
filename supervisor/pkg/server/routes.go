package server

import "net/http"

// routes registers all supervisor API routes on a new ServeMux.
//
// Routes mirror the templates in api.RouteBuilder but use Go 1.22 path
// parameter syntax ({name}) instead of fmt verbs.
func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /v1/health", s.handleHealth)

	// Space data CRUD
	mux.HandleFunc("GET /v1/spaces", s.handleListSpaces)
	mux.HandleFunc("POST /v1/spaces", s.handleCreateSpace)
	mux.HandleFunc("GET /v1/spaces/{name}", s.handleGetSpace)
	mux.HandleFunc("DELETE /v1/spaces/{name}", s.handleDeleteSpace)

	// Quarkfile (versioned)
	mux.HandleFunc("GET /v1/spaces/{name}/quarkfile", s.handleGetQuarkfile)
	mux.HandleFunc("PUT /v1/spaces/{name}/quarkfile", s.handleUpdateQuarkfile)

	// Doctor
	mux.HandleFunc("POST /v1/spaces/{name}/doctor", s.handleDoctor)

	// KB
	mux.HandleFunc("GET /v1/spaces/{name}/kb/{namespace}", s.handleKBList)
	mux.HandleFunc("GET /v1/spaces/{name}/kb/{namespace}/{key}", s.handleKBGet)
	mux.HandleFunc("PUT /v1/spaces/{name}/kb/{namespace}/{key}", s.handleKBSet)
	mux.HandleFunc("DELETE /v1/spaces/{name}/kb/{namespace}/{key}", s.handleKBDelete)

	// Plugins
	mux.HandleFunc("GET /v1/spaces/{name}/plugins", s.handleListPlugins)
	mux.HandleFunc("POST /v1/spaces/{name}/plugins", s.handleInstallPlugin)
	mux.HandleFunc("GET /v1/spaces/{name}/plugins/search", s.handleSearchPlugins)
	mux.HandleFunc("GET /v1/spaces/{name}/plugins/hub/{plugin}", s.handleHubPluginInfo)
	mux.HandleFunc("GET /v1/spaces/{name}/plugins/{plugin}", s.handleGetPlugin)
	mux.HandleFunc("DELETE /v1/spaces/{name}/plugins/{plugin}", s.handleUninstallPlugin)

	// Sessions
	mux.HandleFunc("GET /v1/spaces/{name}/sessions", s.handleListSessions)
	mux.HandleFunc("POST /v1/spaces/{name}/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /v1/spaces/{name}/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("DELETE /v1/spaces/{name}/sessions/{id}", s.handleDeleteSession)

	// Supervisor → agent event stream
	mux.HandleFunc("GET /v1/spaces/{name}/events/stream", s.handleEventStream)

	// Agents (runtime)
	mux.HandleFunc("GET /v1/agents", s.handleListAgents)
	mux.HandleFunc("POST /v1/agents", s.handleStartAgent)
	mux.HandleFunc("GET /v1/agents/{id}", s.handleGetAgent)
	mux.HandleFunc("POST /v1/agents/{id}/stop", s.handleStopAgent)

	return mux
}
