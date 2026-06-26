// Package handler — node endpoints (list + get + lifecycle).
package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/quark/server/internal/query"
)

// NodeHandler handles /api/v1/namespaces/:ns/systems/:sys/nodes endpoints.
type NodeHandler struct {
	nodeSvc *query.NodeQueryService
	lcSvc   *query.LifecycleService
}

// NewNodeHandler constructs a NodeHandler.
func NewNodeHandler(nodeSvc *query.NodeQueryService, lcSvc *query.LifecycleService) *NodeHandler {
	return &NodeHandler{nodeSvc: nodeSvc, lcSvc: lcSvc}
}

// Register wires the node routes onto the given Fiber router.
func (h *NodeHandler) Register(r fiber.Router) {
	r.Get("/", h.list)
	r.Get("/:name", h.get)
	r.Post("/:name/pause", h.transition("pause"))
	r.Post("/:name/resume", h.transition("resume"))
	r.Post("/:name/drain", h.transition("drain"))
	r.Post("/:name/archive", h.transition("archive"))
	r.Post("/:name/recover", h.transition("recover"))
}

func (h *NodeHandler) list(c *fiber.Ctx) error {
	ns := c.Params("namespace")
	sys := c.Params("system")
	nodes, err := h.nodeSvc.ListNodes(c.Context(), ns, sys)
	if err != nil {
		return fiberError(c, 500, err)
	}
	return c.JSON(nodes)
}

func (h *NodeHandler) get(c *fiber.Ctx) error {
	ns := c.Params("namespace")
	sys := c.Params("system")
	name := c.Params("name")
	detail, err := h.nodeSvc.GetNode(c.Context(), ns, sys, name)
	if err != nil {
		if errors.Is(err, query.ErrNotFound) {
			return c.SendStatus(404)
		}
		return fiberError(c, 500, err)
	}
	return c.JSON(detail)
}

func (h *NodeHandler) transition(op string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ns := c.Params("namespace")
		sys := c.Params("system")
		name := c.Params("name")
		err := h.lcSvc.Transition(c.Context(), ns, sys, name, op)
		if err != nil {
			if errors.Is(err, query.ErrNotFound) {
				return c.SendStatus(404)
			}
			if errors.Is(err, query.ErrInvalidTransition) {
				return c.Status(409).JSON(fiber.Map{"message": err.Error()})
			}
			return fiberError(c, 500, err)
		}
		return c.SendStatus(204)
	}
}
