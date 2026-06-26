// Package handler — health endpoints (/health/live and /health/ready).
package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/quark/server/internal/health"
)

// HealthHandler handles /health/live and /health/ready.
type HealthHandler struct {
	checker *health.Checker
}

// NewHealthHandler constructs a HealthHandler.
func NewHealthHandler(checker *health.Checker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

// Register wires the health routes onto the given Fiber router.
func (h *HealthHandler) Register(app *fiber.App) {
	app.Get("/health/live", h.live)
	app.Get("/health/ready", h.ready)
}

func (h *HealthHandler) live(c *fiber.Ctx) error {
	return c.JSON(h.checker.Live())
}

func (h *HealthHandler) ready(c *fiber.Ctx) error {
	return c.JSON(h.checker.Ready(c.Context()))
}
