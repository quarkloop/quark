package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/agent/pkg/channel"
)

// ChannelHandler holds channel handler dependencies.
type ChannelHandler struct {
	bus *channel.ChannelBus
}

// NewChannelHandler creates a new ChannelHandler.
func NewChannelHandler(bus *channel.ChannelBus) *ChannelHandler {
	return &ChannelHandler{bus: bus}
}

// RegisterRoutes wires channel routes onto the given Fiber router.
// The group is expected to be mounted at /v1/channels.
func (h *ChannelHandler) RegisterRoutes(g fiber.Router) {
	g.Get("", h.ListChannels)
}

// ListChannels handles GET /v1/channels.
func (h *ChannelHandler) ListChannels(c *fiber.Ctx) error {
	active := h.bus.ActiveChannels()
	available := h.bus.AvailableChannels()

	return c.JSON(fiber.Map{
		"active":    active,
		"available": available,
	})
}
