// Package nats provides the Go control plane's NATS connection wrapper.
//
// The connection is a hard dependency — the server refuses to start
// without NATS. The wrapper exposes only Connect(); the rest of the
// codebase talks to a plain *nats.Conn.
//
// Reconnect policy: infinite retries with 1s backoff. Same as the
// Catalog's natsx package — the control plane must survive a NATS
// restart without operator intervention.
package nats

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Connect dials NATS at url with the control plane's retry policy.
// Returns the live *nats.Conn on success, or an error wrapping the
// underlying dial failure.
func Connect(url string, log *zap.Logger) (*nats.Conn, error) {
	opts := nats.Options{
		Url:            url,
		MaxReconnect:   -1,
		ReconnectWait:  1 * time.Second,
		Timeout:        5 * time.Second,
		AllowReconnect: true,
		ClosedCB: func(_ *nats.Conn) {
			log.Error("NATS connection closed")
		},
		DisconnectedCB: func(_ *nats.Conn) {
			log.Warn("NATS disconnected, will retry...")
		},
		ReconnectedCB: func(nc *nats.Conn) {
			log.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		},
	}

	nc, err := opts.Connect()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to NATS at %s: %w", url, err)
	}
	return nc, nil
}
