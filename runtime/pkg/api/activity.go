package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/runtime/pkg/activity"
)

// ActivityHandler serves runtime activity records.
type ActivityHandler struct {
	store *activity.Store
}

// NewActivityHandler creates an activity handler.
func NewActivityHandler(store *activity.Store) *ActivityHandler {
	return &ActivityHandler{store: store}
}

// RegisterRoutes wires activity routes onto /v1/activity.
func (h *ActivityHandler) RegisterRoutes(g fiber.Router) {
	g.Get("", h.List)
	g.Get("/stream", h.Stream)
}

// List handles GET /v1/activity.
func (h *ActivityHandler) List(c *fiber.Ctx) error {
	if h.store == nil {
		return c.JSON([]activityResponse{})
	}
	limit := parseLimit(c.Query("limit"))
	records := h.store.List(limit)
	out := make([]activityResponse, 0, len(records))
	for _, record := range records {
		out = append(out, mapActivityResponse(record))
	}
	return c.JSON(out)
}

// Stream handles GET /v1/activity/stream.
func (h *ActivityHandler) Stream(c *fiber.Ctx) error {
	if h.store == nil {
		return c.Status(fiber.StatusNotFound).JSON(errorResponse{Error: "activity store not configured"})
	}
	ch := h.store.Subscribe()
	defer h.store.Unsubscribe(ch)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	ctx := c.Context()
	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		enc := json.NewEncoder(w)
		for {
			select {
			case record, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprint(w, "event: activity\ndata: ")
				if err := enc.Encode(mapActivityResponse(record)); err != nil {
					return
				}
				fmt.Fprint(w, "\n")
				w.Flush()
			case <-ctx.Done():
				return
			}
		}
	})
	return nil
}

func parseLimit(value string) int {
	if value == "" {
		return 0
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 0 {
		return 0
	}
	return limit
}
