//go:build plugin

// Plugin export for lib-mode loading.
// Build with: go build -buildmode=plugin -tags plugin -o plugin.so ./
package main

import "github.com/quarkloop/pkg/plugin"

// NewPlugin creates a new OpenAIProvider instance for lib-mode loading.
func NewPlugin() plugin.Plugin {
	return &OpenAIProvider{}
}
