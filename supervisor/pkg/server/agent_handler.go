package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/registry"
	"github.com/quarkloop/supervisor/pkg/space"
)

// handleListAgents serves GET /v1/agents.
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents := s.agents.List()
	out := make([]api.AgentInfo, 0, len(agents))
	for _, a := range agents {
		out = append(out, toAPIAgentInfo(a))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleStartAgent serves POST /v1/agents.
func (s *Server) handleStartAgent(w http.ResponseWriter, r *http.Request) {
	var req api.StartAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Space == "" {
		writeError(w, http.StatusBadRequest, "space is required")
		return
	}
	if req.WorkingDir == "" {
		writeError(w, http.StatusBadRequest, "working_dir is required")
		return
	}

	if _, err := s.store.Get(req.Space); err != nil {
		if errors.Is(err, space.ErrNotFound) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("space %q not found", req.Space))
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing, err := s.agents.GetBySpace(req.Space); err == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("agent %s already running for space %q", existing.ID, req.Space))
		return
	}

	port := req.Port
	if port == 0 {
		p, err := reservePort()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "reserve port: "+err.Error())
			return
		}
		port = p
	}

	pluginsDir := ""
	if mgr, err := s.store.Plugins(req.Space); err == nil {
		pluginsDir = mgr.PluginsDir()
	}

	agent := &registry.Agent{
		ID:         generateAgentID(),
		Space:      req.Space,
		WorkingDir: req.WorkingDir,
		PluginsDir: pluginsDir,
		Status:     api.AgentStarting,
		Port:       port,
		StartedAt:  time.Now(),
	}
	s.agents.Register(agent)

	if err := s.launcher.Start(r.Context(), agent); err != nil {
		s.agents.Remove(agent.ID)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toAPIAgentInfo(agent))
}

// handleGetAgent serves GET /v1/agents/{id}.
func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := s.agents.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIAgentInfo(agent))
}

// handleStopAgent serves POST /v1/agents/{id}/stop.
func (s *Server) handleStopAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := s.agents.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if agent.Status != api.AgentRunning && agent.Status != api.AgentStarting {
		writeError(w, http.StatusConflict, fmt.Sprintf("agent %s is not running (status: %s)", id, agent.Status))
		return
	}
	if err := s.launcher.Stop(agent); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIAgentInfo(agent))
}

func toAPIAgentInfo(a *registry.Agent) api.AgentInfo {
	info := api.AgentInfo{
		ID:         a.ID,
		Space:      a.Space,
		WorkingDir: a.WorkingDir,
		Status:     a.Status,
		PID:        a.PID,
		Port:       a.Port,
		StartedAt:  a.StartedAt,
	}
	if a.Status == api.AgentRunning && !a.StartedAt.IsZero() {
		info.Uptime = time.Since(a.StartedAt).Round(time.Second).String()
	}
	return info
}

// generateAgentID returns a short random hex ID.
func generateAgentID() string {
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
