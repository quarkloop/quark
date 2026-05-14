package api

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/runtime/pkg/plan"
)

// PlanHandler serves the runtime's current work plan.
type PlanHandler struct {
	plan *plan.Plan
}

// NewPlanHandler creates a plan handler.
func NewPlanHandler(plan *plan.Plan) *PlanHandler {
	return &PlanHandler{plan: plan}
}

// RegisterRoutes wires plan routes onto /v1/plan.
func (h *PlanHandler) RegisterRoutes(g fiber.Router) {
	g.Get("", h.Get)
	g.Post("/approve", h.Approve)
	g.Post("/reject", h.Reject)
}

// Get handles GET /v1/plan.
func (h *PlanHandler) Get(c *fiber.Ctx) error {
	return c.JSON(h.response())
}

// Approve resumes a paused plan. The runtime plan has no separate persisted
// approval entity, so approval maps to "continue execution".
func (h *PlanHandler) Approve(c *fiber.Ctx) error {
	if h.plan != nil {
		h.plan.Resume()
	}
	return c.JSON(h.response())
}

// Reject pauses the current plan so it will not continue executing.
func (h *PlanHandler) Reject(c *fiber.Ctx) error {
	if h.plan != nil {
		h.plan.Pause()
	}
	return c.JSON(h.response())
}

func (h *PlanHandler) response() planResponse {
	if h.plan == nil {
		now := time.Now().UTC()
		return planResponse{
			Goal:      "No active plan",
			Status:    "idle",
			Complete:  true,
			Summary:   "No active work.",
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	now := time.Now().UTC()
	steps := h.plan.GetSteps()
	out := planResponse{
		Goal:      "Current runtime plan",
		Status:    mapPlanStatus(h.plan.GetStatus()),
		Steps:     make([]planStepResponse, 0, len(steps)),
		Complete:  h.plan.GetStatus() == "completed" || h.plan.GetStatus() == "idle",
		Summary:   h.plan.GetSummary(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	for i, step := range steps {
		out.Steps = append(out.Steps, planStepResponse{
			ID:          stepID(i),
			Agent:       "main",
			Description: step.Description(),
			Status:      mapStepStatus(step.Status()),
			Result:      step.Result(),
		})
	}
	return out
}

func mapPlanStatus(status string) string {
	switch status {
	case "active":
		return "executing"
	case "paused":
		return "draft"
	case "completed":
		return "succeeded"
	case "failed":
		return "failed"
	default:
		return status
	}
}

func mapStepStatus(status string) string {
	switch status {
	case "active":
		return "running"
	case "completed":
		return "complete"
	default:
		return status
	}
}

func stepID(index int) string {
	return "step-" + strconv.Itoa(index+1)
}
