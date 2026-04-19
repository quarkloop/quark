package system

import (
	"github.com/gin-gonic/gin"
)

// Handler holds the system handler dependencies.
type Handler struct{}

// NewHandler creates a new Handler.
func NewHandler() *Handler {
	return &Handler{}
}

// RegisterRoutes wires system routes.
func (h *Handler) RegisterRoutes(g *gin.Engine, v1 *gin.RouterGroup) {
	g.GET("/health", h.Health)
	v1.POST("/stop", h.Stop)
}
