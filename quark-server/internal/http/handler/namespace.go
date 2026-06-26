// Package handler — namespace endpoints.
package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/quark/server/internal/query"
)

// NamespaceHandler handles /api/v1/namespaces endpoints.
type NamespaceHandler struct {
	nsSvc *query.NamespaceQueryService
}

// NewNamespaceHandler constructs a NamespaceHandler.
func NewNamespaceHandler(nsSvc *query.NamespaceQueryService) *NamespaceHandler {
	return &NamespaceHandler{nsSvc: nsSvc}
}

// Register wires the namespace routes onto the given Fiber router.
func (h *NamespaceHandler) Register(r fiber.Router) {
	r.Get("/", h.list)
	r.Get("/:ns", h.get)
}

func (h *NamespaceHandler) list(c *fiber.Ctx) error {
	namespaces, err := h.nsSvc.ListNamespaces(c.Context())
	if err != nil {
		return fiberError(c, 500, err)
	}
	return c.JSON(namespaces)
}

func (h *NamespaceHandler) get(c *fiber.Ctx) error {
	ns := c.Params("ns")
	detail, err := h.nsSvc.GetNamespace(c.Context(), ns)
	if err != nil {
		if errors.Is(err, query.ErrNotFound) {
			return c.SendStatus(404)
		}
		return fiberError(c, 500, err)
	}
	return c.JSON(detail)
}
