// Package web provides the HTTP channel — manages the Gin server
// and registers all API routes. No handler code lives here.
package web

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/channel"
	"github.com/quarkloop/agent/pkg/message"
	"github.com/quarkloop/agent/pkg/system"
)

// WebChannel manages the HTTP server and registers API routes.
type WebChannel struct {
	engine *gin.Engine
	addr   string
	srv    *http.Server
}

// New creates a new web channel with all API routes registered.
func New(addr string, a *agent.Agent) *WebChannel {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/v1")
	system.NewHandler().RegisterRoutes(r, v1)
	agent.NewHandler(a).RegisterRoutes(v1)
	// Sessions are owned by the supervisor; the agent only exposes the
	// operational message API for sessions created upstream.
	message.NewHandler(a, a.Sessions).RegisterRoutes(v1.Group("/sessions/:session_id/messages"))

	// Channel API — registered after bus is available via agent
	if a.Bus != nil {
		channel.NewHandler(a.Bus).RegisterRoutes(v1.Group("/channels"))
	}

	return &WebChannel{engine: r, addr: addr}
}

// Type returns the channel type.
func (c *WebChannel) Type() channel.ChannelType { return channel.WebChannelType }

// Start starts the HTTP server.
func (c *WebChannel) Start(ctx context.Context) error {
	c.srv = &http.Server{
		Addr:              c.addr,
		Handler:           c.engine,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		fmt.Printf("web channel listening on %s\n", c.addr)
		if err := c.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("web channel error: %v\n", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (c *WebChannel) Stop(ctx context.Context) error {
	if c.srv == nil {
		return nil
	}
	return c.srv.Shutdown(ctx)
}
