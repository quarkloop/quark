package api

import (
	"github.com/gofiber/fiber/v2"
)

// SystemHandler holds the system handler dependencies.
type SystemHandler struct{}

// NewSystemHandler creates a new SystemHandler.
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

// RegisterRoutes wires system routes onto the given Fiber router.
func (h *SystemHandler) RegisterRoutes(g fiber.Router) {
	g.Get("/health", h.Health)
	g.Post("/stop", h.Stop)
}

// Health handles GET /health.
func (h *SystemHandler) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// Stop handles POST /v1/stop.
func (h *SystemHandler) Stop(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "stopping"})
}
