// Package config loads the Go server's configuration from environment
// variables (with sensible defaults). The server has no config files —
// per the task requirements, all knobs are env-vars.
//
// Conventions:
//   - Every knob has a default that works for `make run-example` locally.
//   - The QUARK_ prefix is reserved for Quark-controlled vars
//     (QUARK_HTTP_PORT, QUARK_NATS_URL, etc.).
//   - Production deployments override the defaults via the process
//     environment (systemd, Kubernetes, docker run -e, etc.).
package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Config is the full runtime configuration of the Go control plane.
//
// All fields are populated from env vars with the prefix QUARK_ on
// startup. The struct is plain data — no methods with side effects.
type Config struct {
	// HTTP port for the REST API (default 8080).
	HTTPPort int `envconfig:"HTTP_PORT" default:"8080"`

	// NATS URL (default nats://localhost:4222).
	NATSURL string `envconfig:"NATS_URL" default:"nats://localhost:4222"`

	// State root: filesystem path used for the catalog DB, data-plane
	// log files, and any other on-disk state. Default ./quark-state.
	StateRoot string `envconfig:"STATE_ROOT" default:"./quark-state"`

	// Data-plane binary: optional override. When empty, the
	// ProcessManager auto-detects (native first, JVM jar fallback).
	DataPlaneBinary string `envconfig:"DATAPLANE_BINARY" default:""`

	// Data-plane port base: the first data-plane process binds to this
	// port; each subsequent one increments. Default 9100.
	DataPlanePortBase int `envconfig:"DATAPLANE_PORT_BASE" default:"9100"`

	// Log format: "json" (production) or "console" (dev). Default console.
	LogFormat string `envconfig:"LOG_FORMAT" default:"console"`

	// Log level: debug, info, warn, error. Default info.
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
}

// Load reads configuration from env vars (prefix QUARK_) and returns
// a populated Config. Returns an error only if envconfig fails to parse
// a value (e.g. QUARK_HTTP_PORT=not-a-number).
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("QUARK", &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &cfg, nil
}
