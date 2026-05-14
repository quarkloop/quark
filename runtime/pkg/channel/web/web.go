// Package web provides the HTTP channel — manages the Fiber server
// and registers all API routes. No handler code lives here.
package web

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/quarkloop/runtime/pkg/agent"
	"github.com/quarkloop/runtime/pkg/api"
	"github.com/quarkloop/runtime/pkg/channel"
)

// WebChannel manages the HTTP server and registers API routes.
type WebChannel struct {
	app  *fiber.App
	addr string
}

// New creates a new web channel with all API routes registered.
func New(addr string, a *agent.Agent) *WebChannel {
	app := fiber.New(fiber.Config{
		ReadTimeout: 10 * time.Second,
		// Message endpoints stream LLM/tool progress over SSE. A model may take
		// longer than a short HTTP write timeout before its first token or tool
		// call, so writes must not be bounded by the generic request timeout.
		WriteTimeout: 0,
		ErrorHandler: errorHandler,
	})

	app.Use(recover.New())

	v1 := app.Group("/v1")
	api.NewSystemHandler().RegisterRoutes(v1)
	api.NewAgentHandler(a).RegisterRoutes(v1)
	api.NewActivityHandler(a.Activity).RegisterRoutes(v1.Group("/activity"))
	api.NewPlanHandler(a.Plan).RegisterRoutes(v1.Group("/plan"))

	// Sessions are owned by the supervisor; the agent only exposes the
	// operational message API for sessions created upstream.
	msgGroup := v1.Group("/sessions/:session_id/messages")
	api.NewMessageHandler(a, a.Sessions).RegisterRoutes(msgGroup)

	// Channel API — registered after bus is available via agent
	if a.Bus != nil {
		chGroup := v1.Group("/channels")
		api.NewChannelHandler(a.Bus).RegisterRoutes(chGroup)
	}

	return &WebChannel{app: app, addr: addr}
}

// Type returns the channel type.
func (c *WebChannel) Type() channel.ChannelType { return channel.WebChannelType }

// Start starts the HTTP server.
func (c *WebChannel) Start(ctx context.Context) error {
	go func() {
		fmt.Printf("web channel listening on %s\n", c.addr)
		if err := c.app.Listen(c.addr); err != nil {
			fmt.Printf("web channel error: %v\n", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (c *WebChannel) Stop(ctx context.Context) error {
	return c.app.ShutdownWithContext(ctx)
}

// errorHandler is the Fiber custom error handler.
func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(errorResponse{Error: err.Error()})
}

type errorResponse struct {
	Error string `json:"error"`
}
