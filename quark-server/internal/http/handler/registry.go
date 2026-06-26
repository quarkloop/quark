// Package handler — registry endpoints (list + lookup) for built-in
// node descriptors.
package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/quark/server/internal/query"
)

// RegistryHandler handles /api/v1/registry endpoints.
type RegistryHandler struct {
	regSvc *query.RegistryQueryService
}

// NewRegistryHandler constructs a RegistryHandler.
func NewRegistryHandler(regSvc *query.RegistryQueryService) *RegistryHandler {
	return &RegistryHandler{regSvc: regSvc}
}

// Register wires the registry routes onto the given Fiber router.
//
// Note: /api/v1/registry/:uri uses Fiber's "*" wildcard to capture
// the full URI (which may contain slashes, e.g. quark/time/schedule/timer:v1).
func (h *RegistryHandler) Register(r fiber.Router) {
	r.Get("/", h.list)
	r.Get("/*", h.lookup)
}

func (h *RegistryHandler) list(c *fiber.Ctx) error {
	q := c.Query("q")
	entries, err := h.regSvc.List(c.Context(), q)
	if err != nil {
		return fiberError(c, 500, err)
	}
	return c.JSON(entries)
}

func (h *RegistryHandler) lookup(c *fiber.Ctx) error {
	uri := c.Params("*")
	if uri == "" {
		return c.SendStatus(404)
	}
	entry, err := h.regSvc.Lookup(c.Context(), uri)
	if err != nil {
		if errors.Is(err, query.ErrNotFound) {
			return c.SendStatus(404)
		}
		return fiberError(c, 500, err)
	}
	return c.JSON(entry)
}
