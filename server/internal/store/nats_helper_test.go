// Package store — test helper that starts an in-process nats-server.
//
// Same pattern as the Catalog's internal test helpers. We import
// nats-server's server package directly (it's already a transitive
// dep of nats.go).
package store

import (
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// runNATSServer starts an in-process nats-server on a random port,
// waits for it to be ready, and returns the *server.Server. The
// caller is responsible for calling Shutdown() (typically via
// t.Cleanup).
func runNATSServer(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		Host:           "127.0.0.1",
		Port:           -1, // pick a random free port
		NoLog:          true,
		NoSigs:         true,
		MaxPayload:     10 * 1024 * 1024,
		WriteDeadline:  10 * time.Second,
	}
	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("nats-server NewServer: %v", err)
	}
	if os.Getenv("QUARK_TEST_NATS_VERBOSE") == "1" {
		s.ConfigureLogger()
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatalf("nats-server did not become ready in 5s")
	}
	return s
}
