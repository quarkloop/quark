package channel

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AllChannelTypes is the list of all known channel types.
var AllChannelTypes = []ChannelType{
	WebChannelType,
	TelegramChannelType,
}

// ListChannels handles GET /v1/channels
func (h *Handler) ListChannels(c *gin.Context) {
	active := h.bus.ActiveChannels()
	available := h.bus.AvailableChannels()

	c.JSON(http.StatusOK, gin.H{
		"active":    active,
		"available": available,
	})
}
