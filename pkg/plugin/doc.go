// Package plugin defines shared plugin interfaces, types, manifest parsing, and loader.
//
// It provides the Plugin interface (ToolPlugin, ProviderPlugin) along with
// manifest parsing (manifest.yaml) and a loader that supports both lib mode
// (.so via plugin.Open) and api mode (HTTP server processes).
package plugin
