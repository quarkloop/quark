package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/agent/pkg/agent"
)

// AgentHandler holds the agent info handler dependencies.
type AgentHandler struct {
	agent *agent.Agent
}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler(a *agent.Agent) *AgentHandler {
	return &AgentHandler{agent: a}
}

// RegisterRoutes wires agent routes onto the given Fiber router.
func (h *AgentHandler) RegisterRoutes(g fiber.Router) {
	g.Get("/info", h.Info)
}

// Info handles GET /v1/info.
func (h *AgentHandler) Info(c *fiber.Ctx) error {
	var busInfo any
	if h.agent.Bus != nil {
		busInfo = h.agent.Bus.ActiveChannels()
	}

	return c.JSON(fiber.Map{
		"id":           h.agent.ID,
		"sessions":     len(h.agent.Sessions.List()),
		"work_status":  h.agent.Plan.GetStatus(),
		"default_model": h.agent.Models.Default,
		"models":       h.agent.Models.List(),
		"channels":     busInfo,
	})
}
