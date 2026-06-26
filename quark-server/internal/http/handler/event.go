// Package handler — event endpoints (list + count).
package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/quark/server/internal/query"
)

// EventHandler handles /api/v1/namespaces/:ns/events endpoints.
type EventHandler struct {
	evtSvc *query.EventQueryService
}

// NewEventHandler constructs an EventHandler.
func NewEventHandler(evtSvc *query.EventQueryService) *EventHandler {
	return &EventHandler{evtSvc: evtSvc}
}

// Register wires the event routes onto the given Fiber router.
func (h *EventHandler) Register(r fiber.Router) {
	r.Get("/", h.list)
	r.Get("/count", h.count)
}

func (h *EventHandler) list(c *fiber.Ctx) error {
	ns := c.Params("namespace")
	system := c.Query("system")
	node := c.Query("node")
	kinds := c.Query("kinds")
	limitStr := c.Query("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}
	events, err := h.evtSvc.Query(c.Context(), ns, system, node, kinds, limit)
	if err != nil {
		return fiberError(c, 500, err)
	}
	return c.JSON(events)
}

func (h *EventHandler) count(c *fiber.Ctx) error {
	ns := c.Params("namespace")
	system := c.Query("system")
	node := c.Query("node")
	kinds := c.Query("kinds")
	n, err := h.evtSvc.Count(c.Context(), ns, system, node, kinds)
	if err != nil {
		return fiberError(c, 500, err)
	}
	return c.JSON(fiber.Map{"count": n})
}
