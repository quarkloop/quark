//go:build plugin

// Plugin export for lib-mode loading.
// Build with: go build -buildmode=plugin -tags plugin -o plugin.so ./
package main

import "github.com/quarkloop/pkg/plugin"

// QuarkPlugin is the exported plugin instance for lib-mode loading.
var QuarkPlugin plugin.Plugin = &OpenRouterProvider{}
