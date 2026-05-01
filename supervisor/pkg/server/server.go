// Package server implements the Supervisor HTTP API.
package server

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/events"
	"github.com/quarkloop/supervisor/pkg/registry"
	"github.com/quarkloop/supervisor/pkg/runtime"
	"github.com/quarkloop/supervisor/pkg/space"
	"github.com/quarkloop/supervisor/pkg/space/fsstore"
)

// Config holds supervisor server configuration.
type Config struct {
	// Port is the TCP port to listen on.
	Port int
	// SpacesDir is the root directory for the filesystem-backed space store.
	// When empty, fsstore.DefaultRoot() is used.
	SpacesDir string
	// AgentBin is the path (or name on $PATH) of the agent binary to launch.
	AgentBin string
}

// Server is the Supervisor HTTP API server.
type Server struct {
	cfg      Config
	store    space.Store
	agents   *registry.Registry
	launcher *runtime.Launcher
	events   *events.Bus
	app      *fiber.App
}

// New creates a new Supervisor server.
func New(cfg Config) (*Server, error) {
	if cfg.Port == 0 {
		cfg.Port = 7200
	}
	if cfg.AgentBin == "" {
		cfg.AgentBin = "agent"
	}
	root := cfg.SpacesDir
	if root == "" {
		r, err := fsstore.DefaultRoot()
		if err != nil {
			return nil, fmt.Errorf("resolve spaces root: %w", err)
		}
		root = r
	}
	store, err := fsstore.NewFSStore(root)
	if err != nil {
		return nil, fmt.Errorf("open space store: %w", err)
	}

	supervisorURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	agentsReg := registry.New()
	agentsLauncher := runtime.New(cfg.AgentBin, supervisorURL, func(id string) {
		agentsReg.SetStopped(id)
	})
	s := &Server{
		cfg:      cfg,
		store:    store,
		agents:   agentsReg,
		launcher: agentsLauncher,
		events:   events.NewBus(),
	}
	s.app = fiber.New(fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorHandler: s.errorHandler,
	})

	s.app.Use(recover.New())
	s.app.Use(logger.New(logger.Config{
		Format: "${time} ${status} - ${latency} ${method} ${path}\n",
	}))
	s.app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	s.routes()
	return s, nil
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("supervisor listening on :%d\n", s.cfg.Port)
		if err := s.app.Listen(fmt.Sprintf(":%d", s.cfg.Port)); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.app.ShutdownWithContext(shutCtx)
	case err := <-errCh:
		return err
	}
}

// errorHandler is the Fiber custom error handler.
func (s *Server) errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(api.ErrorResponse{Error: err.Error()})
}
