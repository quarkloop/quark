package agent

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Info handles GET /v1/info
func (h *Handler) Info(c *gin.Context) {
	var busInfo interface{}
	if h.agent.Bus != nil {
		busInfo = h.agent.Bus.ActiveChannels()
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            h.agent.ID,
		"sessions":      len(h.agent.Sessions.List()),
		"work_status":   h.agent.Plan.GetStatus(),
		"default_model": h.agent.Models.Default,
		"models":        h.agent.Models.List(),
		"channels":      busInfo,
	})
}
