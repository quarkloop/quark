// Package config holds the runtime configuration for the Catalog service.
//
// Configuration is sourced from command-line flags (and may be extended to
// environment variables in the future). Keeping it in a dedicated package
// means the entry point in cmd/quark-catalog stays small and the rest of
// the codebase depends on a typed Config struct rather than scattered
// string flags.
package config

import "flag"

// Config is the resolved configuration for a Catalog instance.
type Config struct {
	// NATSURL is the NATS server URL the Catalog connects to.
	NATSURL string
	// StateRoot is the directory holding catalog.db and any legacy
	// systems/ tree that will be migrated on first startup.
	StateRoot string
}

// FromFlags parses the standard Catalog command-line flags and returns
// a resolved Config. Calls flag.Parse() if it has not been called yet.
func FromFlags() Config {
	natsURL := flag.String("nats", "nats://localhost:4222", "NATS server URL")
	stateRoot := flag.String("state", "./quark-state", "State root directory")
	flag.Parse()
	return Config{NATSURL: *natsURL, StateRoot: *stateRoot}
}
