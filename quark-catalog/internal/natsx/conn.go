// Package natsx provides NATS connection helpers for the Catalog service.
//
// "natsx" (nats-extended) avoids colliding with the upstream nats.go
// package name. The package deliberately exposes only Connect — the
// rest of the codebase talks to a plain *nats.Conn.
package natsx

import (
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// Connect dials the NATS server at url with the retry policy the Catalog
// service needs (infinite reconnects, 1s backoff). NATS is a hard
// dependency — the Catalog refuses to start without it.
//
// The returned connection has lifecycle callbacks wired to the standard
// log package so operators can observe disconnect/reconnect events in
// the catalog log.
func Connect(url string) (*nats.Conn, error) {
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
		return nil, fmt.Errorf("cannot connect to NATS at %s: %w", url, err)
	}
	return nc, nil
}
