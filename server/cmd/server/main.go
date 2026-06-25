// Package main is the Go control plane entry point.
//
// Responsibilities:
//   - Load configuration from env vars (prefix QUARK_)
//   - Initialize the zap logger (JSON in production, console in dev)
//   - Connect to NATS (hard dependency — refuses to start without it)
//   - Construct all service/repository/handler instances (DI via constructors)
//   - Subscribe the event receiver + metrics collector to their NATS subjects
//   - Start the HTTP server (Fiber) on the configured port
//   - On SIGINT/SIGTERM: stop HTTP server, drain NATS subscriptions,
//     stop all data-plane processes, close NATS connection.
//
// The server has no in-memory state — every request goes straight to
// NATS or to the Catalog. Restart is fast and side-effect-free.
package main

import (
        "context"
        "errors"
        "flag"
        "os"
        "os/signal"
        "syscall"
        "time"

        "go.uber.org/zap"
        "go.uber.org/zap/zapcore"

        "github.com/quarkloop/quark/server/internal/config"
        "github.com/quarkloop/quark/server/internal/dataplane"
        "github.com/quarkloop/quark/server/internal/deploy"
        "github.com/quarkloop/quark/server/internal/event"
        "github.com/quarkloop/quark/server/internal/health"
        "github.com/quarkloop/quark/server/internal/http"
        natsclient "github.com/quarkloop/quark/server/internal/nats"
        "github.com/quarkloop/quark/server/internal/metrics"
        "github.com/quarkloop/quark/server/internal/query"
        "github.com/quarkloop/quark/server/internal/store"
)

func main() {
        // Allow -D style flags for backwards compat with the Java server's
        // CLI args (the run-example.sh script passes -Dquarkus.http.port=8080).
        // We parse and ignore them — actual config comes from env vars.
        flag.Parse()

        cfg, err := config.Load()
        if err != nil {
                panic("config load failed: " + err.Error())
        }

        log := newLogger(cfg.LogFormat, cfg.LogLevel)
        defer func() { _ = log.Sync() }()

        log.Info("=== Quark Go control plane starting ===",
                zap.Int("http_port", cfg.HTTPPort),
                zap.String("nats_url", cfg.NATSURL),
                zap.String("state_root", cfg.StateRoot))

        // 1. Connect to NATS (hard dependency)
        nc, err := natsclient.Connect(cfg.NATSURL, log)
        if err != nil {
                log.Fatal("NATS connect failed", zap.Error(err))
        }
        defer nc.Close()

        // 2. Construct repositories (all implemented by NatsCatalogClient)
        catalog := store.NewNatsCatalogClient(nc)

        // 3. Construct the ProcessManager (spawns data-plane processes)
        procMgr := dataplane.NewProcessManager(
                log, cfg.StateRoot, cfg.NATSURL,
                os.Getenv("JAVA_HOME"), // used to resolve java binary for JVM-mode data planes
                cfg.DataPlaneBinary, cfg.DataPlanePortBase,
        )

        // 4. Construct the deploy service
        deploySvc := deploy.NewDeployService(log, nc, catalog, catalog, catalog, procMgr)

        // 5. Construct the query services
        sysQuery := query.NewSystemQueryService(catalog, catalog)
        nodeQuery := query.NewNodeQueryService(catalog)
        nsQuery := query.NewNamespaceQueryService(catalog, catalog, nil) // metrics wired below
        evtQuery := query.NewEventQueryService(catalog)
        srcQuery := query.NewSourceQueryService(catalog)
        regQuery := query.NewRegistryQueryService(catalog)
        lifeSvc := query.NewLifecycleService(catalog)

        // 6. Construct the event receiver + metrics collector
        evtReceiver := event.New(log, nc, catalog)
        metricsCollector := metrics.New(log, nc)

        // Wire the metrics collector into the namespace query service
        nsQuery = query.NewNamespaceQueryService(catalog, catalog, metricsCollector.AsProvider())

        // 7. Construct the health checker
        checker := health.New(nc)
        checker.SetNATSOK(true)

        // 8. Construct the HTTP server
        httpServer := http.New(http.Deps{
                Logger:           log,
                DeploySvc:        deploySvc,
                SysQuery:         sysQuery,
                NodeQuery:        nodeQuery,
                NsQuery:          nsQuery,
                EvtQuery:         evtQuery,
                SrcQuery:         srcQuery,
                RegQuery:         regQuery,
                LifeSvc:          lifeSvc,
                Metrics:          metricsCollector,
                Checker:          checker,
                NodeRegistryNATS: nc,
        })

        // 9. Start background subscribers (event + metrics)
        if err := evtReceiver.Start(); err != nil {
                log.Fatal("event receiver start failed", zap.Error(err))
        }
        if err := metricsCollector.Start(); err != nil {
                log.Fatal("metrics collector start failed", zap.Error(err))
        }

        // 10. Recover previously-deployed systems from the Catalog
        go func() {
                ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
                defer cancel()
                recovered := deploySvc.RecoverFromDisk(ctx)
                if len(recovered) > 0 {
                        log.Info("recovered systems", zap.Strings("systems", recovered))
                }
        }()

        // 11. Start the HTTP server in a goroutine
        errCh := make(chan error, 1)
        go func() {
                addr := ":" + itoa(cfg.HTTPPort)
                if err := httpServer.Listen(addr); err != nil {
                        errCh <- err
                }
        }()

        // 12. Wait for SIGINT/SIGTERM or a fatal HTTP error
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

        select {
        case sig := <-sigCh:
                log.Info("received signal, shutting down", zap.String("signal", sig.String()))
        case err := <-errCh:
                log.Error("HTTP server failed", zap.Error(err))
        }

        // 13. Graceful shutdown
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := httpServer.Shutdown(shutdownCtx); err != nil {
                log.Warn("HTTP shutdown error", zap.Error(err))
        }
        metricsCollector.Stop()
        evtReceiver.Stop()
        procMgr.StopAll()

        log.Info("=== Quark Go control plane stopped ===")
}

// newLogger constructs a zap.Logger based on the format/level config.
//   - format: "json" (production) or "console" (dev, default)
//   - level: "debug", "info", "warn", "error" (default info)
func newLogger(format, level string) *zap.Logger {
        var zapLevel zapcore.Level
        switch level {
        case "debug":
                zapLevel = zapcore.DebugLevel
        case "info":
                zapLevel = zapcore.InfoLevel
        case "warn":
                zapLevel = zapcore.WarnLevel
        case "error":
                zapLevel = zapcore.ErrorLevel
        default:
                zapLevel = zapcore.InfoLevel
        }

        var zapConfig zap.Config
        if format == "json" {
                zapConfig = zap.NewProductionConfig()
        } else {
                zapConfig = zap.NewDevelopmentConfig()
                zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
        }
        zapConfig.Level = zap.NewAtomicLevelAt(zapLevel)

        l, err := zapConfig.Build()
        if err != nil {
                // Fall back to a no-frills logger if config fails.
                return zap.NewNop()
        }
        return l
}

// itoa is a tiny strconv.Itoa wrapper to keep main.go self-contained.
// Used only for formatting the HTTP port.
func itoa(n int) string {
        if n == 0 {
                return "0"
        }
        neg := n < 0
        if neg {
                n = -n
        }
        var buf [12]byte
        i := len(buf)
        for n > 0 {
                i--
                buf[i] = byte('0' + n%10)
                n /= 10
        }
        if neg {
                i--
                buf[i] = '-'
        }
        return string(buf[i:])
}

// Ensure errors is referenced — used in fatal startup paths where we
// wrap multiple errors before logging.
var _ = errors.New
