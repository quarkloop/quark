// Package main — NATS connection management for the Catalog service.
package main

import (
        "fmt"
        "log"
        "time"

        "github.com/nats-io/nats.go"
)

// ConnectNATS connects to the NATS server with retry logic.
// NATS is a hard requirement — the catalog refuses to start without it.
func ConnectNATS(url string) (*nats.Conn, error) {
        opts := nats.Options{
                Url:            url,
                MaxReconnect:   -1,
                ReconnectWait:  1 * time.Second,
                Timeout:        5 * time.Second,
                AllowReconnect: true,
                ClosedCB: func(_ *nats.Conn) {
                        log.Printf("[ERROR] NATS connection closed")
                },
                DisconnectedCB: func(_ *nats.Conn) {
                        log.Printf("[WARN] NATS disconnected, will retry...")
                },
                ReconnectedCB: func(nc *nats.Conn) {
                        log.Printf("[INFO] NATS reconnected to %s", nc.ConnectedUrl())
                },
        }

        nc, err := opts.Connect()
        if err != nil {
                return nil, fmt.Errorf("FATAL: Cannot connect to NATS at %s: %w", url, err)
        }
        return nc, nil
}
