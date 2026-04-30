package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/registry"
	"github.com/quarkloop/supervisor/pkg/space/store"
)

// handleListAgents serves GET /v1/agents.
func (s *Server) handleListAgents(c *fiber.Ctx) error {
	agents := s.agents.List()
	out := make([]api.AgentInfo, 0, len(agents))
	for _, a := range agents {
		out = append(out, toAPIAgentInfo(a))
	}
	return writeJSON(c, fiber.StatusOK, out)
}

// handleStartAgent serves POST /v1/agents.
func (s *Server) handleStartAgent(c *fiber.Ctx) error {
	var req api.StartAgentRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid request body: "+err.Error())
	}
	if req.Space == "" {
		return writeError(c, fiber.StatusBadRequest, "space is required")
	}

	port := req.Port
	if port == 0 {
		p, err := reservePort()
		if err != nil {
			return writeError(c, fiber.StatusInternalServerError, "reserve port: "+err.Error())
		}
		port = p
	}

	sp, err := s.store.Get(req.Space)
	if err != nil {
		if store.IsNotFound(err) {
			return writeError(c, fiber.StatusNotFound, fmt.Sprintf("space %q not found", req.Space))
		}
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}
	if existing, err := s.agents.GetBySpace(req.Space); err == nil {
		return writeError(c, fiber.StatusConflict, fmt.Sprintf("agent %s already running for space %q", existing.ID, req.Space))
	}

	pluginsDir := ""
	if mgr, err := s.store.Plugins(req.Space); err == nil {
		pluginsDir = mgr.PluginsDir()
	}

	agent := &registry.Agent{
		ID:         generateAgentID(),
		Space:      req.Space,
		WorkingDir: sp.WorkingDir,
		PluginsDir: pluginsDir,
		Status:     api.AgentStarting,
		Port:       port,
		StartedAt:  time.Now(),
	}
	s.agents.Register(agent)

	env, err := s.store.AgentEnvironment(req.Space)
	if err != nil {
		s.agents.Remove(agent.ID)
		return writeError(c, fiber.StatusBadRequest, err.Error())
	}

	if err := s.launcher.Start(c.Context(), agent, env); err != nil {
		s.agents.Remove(agent.ID)
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}

	return writeJSON(c, fiber.StatusCreated, toAPIAgentInfo(agent))
}

// handleGetAgent serves GET /v1/agents/:id.
func (s *Server) handleGetAgent(c *fiber.Ctx) error {
	id := c.Params("id")
	agent, err := s.agents.Get(id)
	if err != nil {
		return writeError(c, fiber.StatusNotFound, err.Error())
	}
	return writeJSON(c, fiber.StatusOK, toAPIAgentInfo(agent))
}

// handleStopAgent serves POST /v1/agents/:id/stop.
func (s *Server) handleStopAgent(c *fiber.Ctx) error {
	id := c.Params("id")
	agent, err := s.agents.Get(id)
	if err != nil {
		return writeError(c, fiber.StatusNotFound, err.Error())
	}
	if agent.Status != api.AgentRunning && agent.Status != api.AgentStarting {
		return writeError(c, fiber.StatusConflict, fmt.Sprintf("agent %s is not running (status: %s)", id, agent.Status))
	}
	if err := s.launcher.Stop(agent); err != nil {
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}
	return writeJSON(c, fiber.StatusOK, toAPIAgentInfo(agent))
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
	if _, err := rand.Read(buf[:]); err != nil {
		// Fallback: use timestamp + pid for uniqueness (not cryptographically secure)
		slog.Error("failed to generate random ID", "error", err)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}
