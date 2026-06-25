// Package http is the Fiber app setup, middleware wiring, and route
// registration.
//
// The Server struct owns the *fiber.App and exposes Listen(addr),
// Shutdown(ctx) for graceful shutdown.
package http

import (
        "context"
        "fmt"
        "net/http"
        "time"

        "github.com/gofiber/fiber/v2"
        "github.com/nats-io/nats.go"
        "go.uber.org/zap"

        "github.com/quarkloop/quark/server/internal/deploy"
        "github.com/quarkloop/quark/server/internal/health"
        "github.com/quarkloop/quark/server/internal/http/handler"
        "github.com/quarkloop/quark/server/internal/http/middleware"
        "github.com/quarkloop/quark/server/internal/metrics"
        "github.com/quarkloop/quark/server/internal/query"
)

// Server is the HTTP server. It owns the *fiber.App and the
// configured handlers.
type Server struct {
        app    *fiber.App
        log    *zap.Logger
}

// Deps bundles the services the HTTP server depends on. Each one is
// constructed in main.go and passed in (DI via constructor).
type Deps struct {
        Logger    *zap.Logger
        DeploySvc *deploy.DeployService
        SysQuery  *query.SystemQueryService
        NodeQuery *query.NodeQueryService
        NsQuery   *query.NamespaceQueryService
        EvtQuery  *query.EventQueryService
        SrcQuery  *query.SourceQueryService
        RegQuery  *query.RegistryQueryService
        LifeSvc   *query.LifecycleService
        Metrics   *metrics.Collector
        Checker   *health.Checker

        // NodeRegistryNATS is the raw *nats.Conn used by the
        // /api/v1/registry/nodes proxy handlers. The handler does not
        // decode the response — it forwards the raw JSON byte-for-byte.
        NodeRegistryNATS *nats.Conn
}

// New constructs a Server with all middleware + routes registered.
func New(deps Deps) *Server {
        app := fiber.New(fiber.Config{
                AppName:      "quark-server",
                ReadTimeout:  30 * time.Second,
                WriteTimeout: 30 * time.Second,
                IdleTimeout:  60 * time.Second,
        })

        // Middleware (order matters):
        //   1. RequestID — first, so all subsequent log lines have an ID
        //   2. Recoverer — catches panics in any handler
        //   3. Logger — logs every request with status + duration
        //   4. CORS — permissive for browser-based CLI tools
        app.Use(middleware.RequestID())
        app.Use(middleware.Recoverer(deps.Logger))
        app.Use(middleware.Logger(deps.Logger))
        app.Use(middleware.CORS())

        // Health endpoints (registered at root, not under /api/v1)
        healthHandler := handler.NewHealthHandler(deps.Checker)
        healthHandler.Register(app)

        // /api/v1 group
        api := app.Group("/api/v1")

        // Namespaces: GET /api/v1/namespaces, GET /api/v1/namespaces/:ns
        nsHandler := handler.NewNamespaceHandler(deps.NsQuery)
        nsHandler.Register(api.Group("/namespaces"))

        // Systems: full CRUD under /api/v1/namespaces/:ns/systems
        sysHandler := handler.NewSystemHandler(deps.DeploySvc, deps.SysQuery, deps.SrcQuery)
        sysHandler.Register(api.Group("/namespaces/:namespace/systems"))

        // Nodes: list/get + lifecycle under /api/v1/namespaces/:ns/systems/:sys/nodes
        nodeHandler := handler.NewNodeHandler(deps.NodeQuery, deps.LifeSvc)
        nodeHandler.Register(api.Group("/namespaces/:namespace/systems/:system/nodes"))

        // Events: list/count under /api/v1/namespaces/:ns/events
        evtHandler := handler.NewEventHandler(deps.EvtQuery)
        evtHandler.Register(api.Group("/namespaces/:namespace/events"))

        // Registry: built-in node descriptors
        // GET /api/v1/registry, GET /api/v1/registry/:uri
        // IMPORTANT: register /api/v1/registry/nodes/* BEFORE the
        // /api/v1/registry/* catch-all, or the catch-all would swallow
        // /nodes and its sub-paths.
        nodeRegHandler := handler.NewNodeRegistryHandler(deps.NodeRegistryNATS)
        nodeRegHandler.Register(api.Group("/registry/nodes"))

        regHandler := handler.NewRegistryHandler(deps.RegQuery)
        regHandler.Register(api.Group("/registry"))

        return &Server{app: app, log: deps.Logger}
}

// Listen starts the HTTP server on addr (e.g. ":8080"). Blocks until
// the server is shut down.
func (s *Server) Listen(addr string) error {
        s.log.Info("HTTP server starting", zap.String("addr", addr))
        return s.app.Listen(addr)
}

// Shutdown gracefully shuts down the server, waiting up to the
// context's deadline for in-flight requests to complete.
func (s *Server) Shutdown(ctx context.Context) error {
        s.log.Info("HTTP server shutting down...")
        if err := s.app.ShutdownWithContext(ctx); err != nil {
                return fmt.Errorf("http shutdown: %w", err)
        }
        s.log.Info("HTTP server stopped")
        return nil
}

// TestClient returns the *fiber.App for use with fiber's testing
// helpers (httptest). Used by tests in *_test.go files.
func (s *Server) TestClient() *fiber.App {
        return s.app
}

// ensure net/http import is used (for the constant timeouts above)
var _ = http.MethodGet
