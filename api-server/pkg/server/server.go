// Package server wires together all api-server subsystems.
//
// Startup sequence:
//  1. Expand and create the data directory.
//  2. Resolve the space-runtime binary path.
//  3. Seed the local registry.
//  4. Open the JSONL space store.
//  5. Create the Controller.
//  6. Mount HTTP handlers (space + repo).
//  7. Start the reconciliation loop and HTTP listener concurrently.
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/quarkloop/agent/pkg/infra/httpserver"
	"github.com/quarkloop/api-server/pkg/repo"
	"github.com/quarkloop/api-server/pkg/space"
)

// Server is the top-level api-server object, owning the HTTP listener,
// JSONL space store, and process controller.
type Server struct {
	cfg        *Config
	store      space.Store
	controller *space.Controller
	http       *httpserver.Server
}

// New constructs and configures a Server from cfg but does not start listening.
// Call Run to begin serving requests.
// New creates and wires the api-server: creates the data directory,
// locates the space-runtime binary, seeds the local registry (idempotent),
// opens the JSONL space store, and registers all HTTP routes.
func New(cfg *Config) (*Server, error) {
	dataDir := expandHome(cfg.DataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}

	runtimeBin, err := resolveRuntimeBin()
	if err != nil {
		return nil, err
	}
	log.Printf("api-server: using agent binary at %s", runtimeBin)

	// Seed the local registry with built-in agent/skill definitions.
	// Safe to call every start — existing files are never overwritten.
	if err := repo.ScaffoldRegistry(); err != nil {
		log.Printf("api-server: warning: registry scaffold failed: %v", err)
	}

	// Space records are stored as individual JSON files under <dataDir>/spaces/.
	store, err := space.OpenStore(filepath.Join(dataDir, "spaces"))
	if err != nil {
		return nil, err
	}

	apiAddr := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
	ctrl := space.NewController(store, runtimeBin, apiAddr, cfg.SpacePortRangeStart, cfg.SpacePortRangeEnd)

	routers := []httpserver.Router{
		space.NewHandler(store, ctrl),
		repo.NewHandler(),
	}

	mux := http.NewServeMux()
	for _, router := range routers {
		router.RegisterRoutes(mux)
	}

	srv := httpserver.New(cfg.Host, cfg.Port, mux)
	return &Server{cfg: cfg, store: store, controller: ctrl, http: srv}, nil
}

// Run starts the Controller reconcile loop and HTTP server concurrently,
// blocking until ctx is cancelled or a fatal error occurs.
func (s *Server) Run(ctx context.Context) error {
	go s.controller.Run(ctx)
	log.Printf("api-server listening on %s:%d", s.cfg.Host, s.cfg.Port)
	errCh := make(chan error, 1)
	go func() { errCh <- s.http.Start() }()
	select {
	case <-ctx.Done():
		return s.http.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// Close flushes and closes the JSONL store. Call after Run returns.
// Close flushes and releases the space store. Call after Run returns.
func (s *Server) Close() error {
	return s.store.Close()
}

func expandHome(path string) string {
	if len(path) > 1 && path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// resolveRuntimeBin finds the space-runtime binary.
// It looks in this order:
//  1. Same directory as the running api-server executable.
//  2. PATH.
func resolveRuntimeBin() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "agent")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath("agent"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("agent binary not found next to api-server or in PATH")
}
