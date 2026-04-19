// Package config provides shared CLI configuration values.
package buildinfo

// Version is the application version, set at build time via ldflags:
//
//	go build -ldflags "-X github.com/quarkloop/cli/pkg/buildinfo.Version=1.0.0"
var Version = "0.1.0"
