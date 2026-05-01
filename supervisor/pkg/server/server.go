// Package server implements the Supervisor HTTP API.
package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/quarkloop/supervisor/internal/supervisor"
	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/events"
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
	// RuntimeBin is the path (or name on $PATH) of the runtime binary to launch.
	RuntimeBin string
}

// Server is the Supervisor HTTP API server.
type Server struct {
	cfg Config
	app *fiber.App

	store    space.Store
	registry *runtime.Registry
	launcher *runtime.Launcher
	events   *events.Bus
}

// New creates a new Supervisor server.
func New(cfg Config) (*Server, error) {
	if cfg.Port == 0 {
		cfg.Port = 7200
	}
	if cfg.RuntimeBin == "" {
		cfg.RuntimeBin = "runtime"
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
	runtimesReg := runtime.NewRegistry()
	runtimesLauncher := runtime.NewLauncher(cfg.RuntimeBin, supervisorURL, func(id string) {
		runtimesReg.SetStopped(id)
	})
	s := &Server{
		cfg:      cfg,
		store:    store,
		registry: runtimesReg,
		launcher: runtimesLauncher,
		events:   events.NewBus(),
	}

	fiberConfig := fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorHandler: s.errorHandler,
	}
	s.app = fiber.New(fiberConfig)
	s.app.Use(recover.New())
	s.app.Use(logger.New(logger.Config{Format: "${time} ${status} - ${latency} ${method} ${path}\n"}))
	s.app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	s.routes()
	return s, nil
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	sup := supervisor.New()
	// Write state before accepting traffic
	if err := sup.Save(supervisor.State{Port: s.cfg.Port, PID: os.Getpid()}); err != nil {
		return fmt.Errorf("writing state: %w", err)
	}
	defer sup.Clear() // clean up on exit

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("supervisor listening on :%d\n", s.cfg.Port)
		if err := s.app.Listen(fmt.Sprintf(":%d", s.cfg.Port)); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
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

func Stop() error {
	sup := supervisor.New()

	state, err := sup.Load()
	if err != nil {
		return err // "supervisor is not running"
	}

	return stopSupervisorProcess(state.PID, state.Port)
}
