// Package main is the Catalog service entry point.
//
// The Catalog is a standalone Go process that provides:
//   - Metadata storage (systems, nodes, events, source, registry) via SQLite
//   - Node package registry (push/pull/list/search .ts and .so files)
//   - JSONL migration on first startup
//
// All communication is via NATS request-reply on catalog.* and
// registry.* subjects. The Java control plane talks to this service
// through the NatsCatalogClient adapter; no Java process opens the
// SQLite database directly.
//
// This file is intentionally small — it only wires together the
// packages under internal/. All logic lives there so it can be
// unit-tested without spinning up the process.
package main

import (
        "log"
        "os"
        "os/signal"
        "syscall"

        "github.com/quarkloop/quark/quark-catalog/internal/config"
        "github.com/quarkloop/quark/quark-catalog/internal/natsx"
        "github.com/quarkloop/quark/quark-catalog/internal/server"
        "github.com/quarkloop/quark/quark-catalog/internal/store"
)

func main() {
        cfg := config.FromFlags()
        log.SetFlags(log.LstdFlags | log.Lmicroseconds)
        log.Printf("[INFO] Quark Catalog starting (nats=%s, state=%s)", cfg.NATSURL, cfg.StateRoot)

        // 1. Open (or create) the SQLite store.
        st, err := store.Open(cfg.StateRoot)
        if err != nil {
                log.Fatalf("[FATAL] Failed to initialize store: %v", err)
        }
        defer st.Close()
        log.Printf("[INFO] SQLite store initialized at %s/catalog.db", cfg.StateRoot)

        // 2. Run legacy JSONL migration if $STATE_ROOT/systems/ exists.
        if migrated, err := st.MigrateLegacy(cfg.StateRoot); err != nil {
                log.Printf("[WARN] Migration error: %v", err)
        } else if migrated.Systems > 0 || migrated.Events > 0 {
                log.Printf("[INFO] Migration complete: %d systems, %d events migrated",
                        migrated.Systems, migrated.Events)
        }

        // 3. Connect to NATS (hard dependency — refuses to start without it).
        nc, err := natsx.Connect(cfg.NATSURL)
        if err != nil {
                log.Fatalf("[FATAL] %v", err)
        }
        defer nc.Close()
        log.Printf("[INFO] Connected to NATS at %s", cfg.NATSURL)

        // 4. Register NATS handlers and start serving.
        srv := server.New(nc, st)
        if err := srv.Start(); err != nil {
                log.Fatalf("[FATAL] Failed to start server: %v", err)
        }
        log.Printf("[INFO] Catalog service ready — listening on catalog.* and registry.* subjects")

        // 5. Block until SIGINT/SIGTERM.
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
        sig := <-sigCh
        log.Printf("[INFO] Received signal %v, shutting down...", sig)
}
