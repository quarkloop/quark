// Package server implements the Supervisor HTTP API.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/quarkloop/supervisor/pkg/events"
	"github.com/quarkloop/supervisor/pkg/registry"
	"github.com/quarkloop/supervisor/pkg/runtime"
	"github.com/quarkloop/supervisor/pkg/space"
)

// Config holds supervisor server configuration.
type Config struct {
	// Port is the TCP port to listen on.
	Port int
	// SpacesDir is the root directory for the filesystem-backed space store.
	// When empty, space.DefaultRoot() is used.
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
	srv      *http.Server
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
		r, err := space.DefaultRoot()
		if err != nil {
			return nil, fmt.Errorf("resolve spaces root: %w", err)
		}
		root = r
	}
	store, err := space.NewFSStore(root)
	if err != nil {
		return nil, fmt.Errorf("open space store: %w", err)
	}

	supervisorURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	s := &Server{
		cfg:      cfg,
		store:    store,
		agents:   registry.New(),
		launcher: runtime.New(cfg.AgentBin, supervisorURL),
		events:   events.NewBus(),
	}
	s.srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s, nil
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("supervisor listening on :%d\n", s.cfg.Port)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}
