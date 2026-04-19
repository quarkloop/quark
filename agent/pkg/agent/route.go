package agent

import (
	"github.com/gin-gonic/gin"
)

// Handler holds the agent system handler dependencies.
type Handler struct {
	agent *Agent
}

// NewHandler creates a new Handler.
func NewHandler(a *Agent) *Handler {
	return &Handler{agent: a}
}

// RegisterRoutes wires agent system routes.
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup) {
	v1.GET("/info", h.Info)
}
