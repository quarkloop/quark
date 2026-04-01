// Package config provides shared CLI configuration values.
package config

import "os"

// DefaultAPIServerAddr is the default api-server listen address.
const DefaultAPIServerAddr = "http://127.0.0.1:7070"

// APIServerOverride is populated by the root command's persistent --api-server
// flag. When non-empty it takes highest precedence over everything else.
var APIServerOverride string

// APIServerURL returns the api-server base URL using this precedence:
//  1. --api-server flag  (APIServerOverride)
//  2. QUARK_API_SERVER   environment variable
//  3. DefaultAPIServerAddr
func APIServerURL() string {
	if APIServerOverride != "" {
		return APIServerOverride
	}
	if url := os.Getenv("QUARK_API_SERVER"); url != "" {
		return url
	}
	return DefaultAPIServerAddr
}

// Version is the application version, set at build time via ldflags:
//
//	go build -ldflags "-X github.com/quarkloop/cli/pkg/cli/config.Version=1.0.0"
var Version = "0.1.0"
