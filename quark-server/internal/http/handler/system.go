// Package handler contains the Fiber HTTP handlers for every REST endpoint.
//
// Each handler is a thin wrapper around a service method:
//   - decode request body (if any)
//   - call service
//   - encode response (or error)
//
// Handlers do NOT call NATS directly — they go through services,
// which go through repositories, which call NATS. Strict layering.
package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/quark/server/internal/deploy"
	"github.com/quarkloop/quark/server/internal/http/dto"
	"github.com/quarkloop/quark/server/internal/query"
)

// SystemHandler handles /api/v1/namespaces/:ns/systems endpoints.
type SystemHandler struct {
	deploySvc *deploy.DeployService
	querySvc  *query.SystemQueryService
	srcSvc    *query.SourceQueryService
}

// NewSystemHandler constructs a SystemHandler.
func NewSystemHandler(deploySvc *deploy.DeployService, querySvc *query.SystemQueryService, srcSvc *query.SourceQueryService) *SystemHandler {
	return &SystemHandler{deploySvc: deploySvc, querySvc: querySvc, srcSvc: srcSvc}
}

// Register wires the system routes onto the given Fiber router.
func (h *SystemHandler) Register(r fiber.Router) {
	r.Get("/", h.list)
	r.Post("/", h.deploy)
	r.Get("/:name", h.get)
	r.Put("/:name", h.apply)
	r.Delete("/:name", h.delete)
	r.Get("/:name/source", h.getSource)
}

func (h *SystemHandler) list(c *fiber.Ctx) error {
	ns := c.Params("namespace")
	systems, err := h.querySvc.ListSystems(c.Context(), ns)
	if err != nil {
		return fiberError(c, 500, err)
	}
	return c.JSON(systems)
}

func (h *SystemHandler) deploy(c *fiber.Ctx) error {
	ns := c.Params("namespace")
	var req dto.DeploySystemRequest
	if err := c.BodyParser(&req); err != nil {
		return fiberError(c, 400, errors.New("invalid JSON body: "+err.Error()))
	}
	if req.Source == "" {
		return fiberError(c, 400, errors.New("source is required"))
	}

	result, err := h.deploySvc.Deploy(c.Context(), req.Source, ns)
	if err != nil {
		if errors.Is(err, deploy.ErrParse) {
			return c.Status(400).JSON(dto.DeploySystemFailure{
				Message: err.Error(),
				Errors: []dto.ValidationError{
					{Path: "system", Message: err.Error(), Severity: "ERROR"},
				},
			})
		}
		return fiberError(c, 500, err)
	}

	nodeNames := make([]string, 0, len(result.Nodes))
	for _, n := range result.Nodes {
		nodeNames = append(nodeNames, n.Name)
	}
	return c.Status(201).JSON(dto.DeploySystemResponse{
		Name:      result.Name,
		Namespace: result.Namespace,
		NodeCount: len(result.Nodes),
		State:     result.State,
		Health:    result.Health,
		Nodes:     nodeNames,
	})
}

func (h *SystemHandler) get(c *fiber.Ctx) error {
	ns := c.Params("namespace")
	name := c.Params("name")
	detail, err := h.querySvc.GetSystem(c.Context(), ns, name)
	if err != nil {
		if errors.Is(err, query.ErrNotFound) {
			return c.SendStatus(404)
		}
		return fiberError(c, 500, err)
	}
	return c.JSON(detail)
}

func (h *SystemHandler) apply(c *fiber.Ctx) error {
	ns := c.Params("namespace")
	_ = c.Params("name") // name is in the URL but we don't use it — the source is authoritative
	var req dto.DeploySystemRequest
	if err := c.BodyParser(&req); err != nil {
		return fiberError(c, 400, errors.New("invalid JSON body: "+err.Error()))
	}
	if req.Source == "" {
		return fiberError(c, 400, errors.New("source is required"))
	}

	result, err := h.deploySvc.Apply(c.Context(), req.Source, ns)
	if err != nil {
		if errors.Is(err, deploy.ErrParse) {
			return c.Status(400).JSON(dto.DeploySystemFailure{
				Message: err.Error(),
				Errors: []dto.ValidationError{
					{Path: "system", Message: err.Error(), Severity: "ERROR"},
				},
			})
		}
		return fiberError(c, 500, err)
	}
	if result.Created {
		return c.Status(201).JSON(result)
	}
	return c.JSON(result)
}

func (h *SystemHandler) delete(c *fiber.Ctx) error {
	ns := c.Params("namespace")
	name := c.Params("name")
	_ = h.deploySvc.Undeploy(c.Context(), ns, name)
	return c.SendStatus(204)
}

func (h *SystemHandler) getSource(c *fiber.Ctx) error {
	ns := c.Params("namespace")
	name := c.Params("name")
	src, err := h.srcSvc.GetSource(c.Context(), ns, name)
	if err != nil {
		if errors.Is(err, query.ErrNotFound) {
			return c.SendStatus(404)
		}
		return fiberError(c, 500, err)
	}
	c.Set("Content-Type", "text/plain")
	return c.SendString(src)
}

// fiberError is a tiny helper that logs the error and returns a 500
// JSON response with the error message.
func fiberError(c *fiber.Ctx, status int, err error) error {
	return c.Status(status).JSON(fiber.Map{
		"code":    "HTTP_ERROR",
		"message": err.Error(),
	})
}
