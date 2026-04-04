// Package config provides shared CLI configuration values.
package config

// Version is the application version, set at build time via ldflags:
//
//	go build -ldflags "-X github.com/quarkloop/cli/pkg/config.Version=1.0.0"
var Version = "0.1.0"
