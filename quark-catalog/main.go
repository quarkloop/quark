// Package main is the entry point for the Quark Catalog service.
//
// The Catalog service is a standalone Go process that provides:
//   - Metadata storage (systems, nodes, events, source, registry) via SQLite
//   - Node package registry (push/pull/list/search .so and .ts files)
//   - JSONL migration on first startup
//
// All communication is via NATS request-reply on catalog.* and registry.* subjects.
package main

import (
        "flag"
        "log"
        "os"
        "os/signal"
        "syscall"
)

func main() {
        natsURL := flag.String("nats", "nats://localhost:4222", "NATS server URL")
        stateRoot := flag.String("state", "./quark-state", "State root directory")
        flag.Parse()

        log.SetFlags(log.LstdFlags | log.Lmicroseconds)
        log.Printf("[INFO] Quark Catalog starting (nats=%s, state=%s)", *natsURL, *stateRoot)

        // Initialize the SQLite store
        store, err := NewStore(*stateRoot)
        if err != nil {
                log.Fatalf("[FATAL] Failed to initialize store: %v", err)
        }
        defer store.Close()
        log.Printf("[INFO] SQLite store initialized at %s/catalog.db", *stateRoot)

        // Run JSONL migration if needed
        migrated, err := store.MigrateLegacy(*stateRoot)
        if err != nil {
                log.Printf("[WARN] Migration error: %v", err)
        } else if migrated.Systems > 0 || migrated.Events > 0 {
                log.Printf("[INFO] Migration complete: %d systems, %d events migrated", migrated.Systems, migrated.Events)
        }

        // Connect to NATS
        nc, err := ConnectNATS(*natsURL)
        if err != nil {
                log.Fatalf("[FATAL] Cannot connect to NATS: %v", err)
        }
        defer nc.Close()
        log.Printf("[INFO] Connected to NATS at %s", *natsURL)

        // Register NATS handlers
        srv := NewServer(nc, store)
        if err := srv.Start(); err != nil {
                log.Fatalf("[FATAL] Failed to start server: %v", err)
        }
        log.Printf("[INFO] Catalog service ready — listening on catalog.* and registry.* subjects")

        // Wait for shutdown signal
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
        sig := <-sigCh
        log.Printf("[INFO] Received signal %v, shutting down...", sig)
}
