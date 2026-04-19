package channel

import (
	"github.com/gin-gonic/gin"
)

// ChannelBusApi is the interface the channel handler needs.
type ChannelBusApi interface {
	ActiveChannels() []ChannelInfo
	AvailableChannels() []ChannelInfo
}

// ChannelInfo describes a channel for the API response.
type ChannelInfo struct {
	Type   ChannelType `json:"type"`
	Active bool        `json:"active"`
}

// Handler holds channel handler dependencies.
type Handler struct {
	bus *ChannelBus
}

// NewHandler creates a new channel Handler.
func NewHandler(bus *ChannelBus) *Handler {
	return &Handler{bus: bus}
}

// RegisterRoutes wires channel routes.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("", h.ListChannels)
}
