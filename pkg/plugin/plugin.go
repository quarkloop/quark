// Package plugin defines the unified plugin interfaces and types for Quark.
//
// Quark supports two plugin modes:
//   - lib mode: plugins compiled as .so files, loaded via Go's plugin system
//   - binary mode: plugins run as separate HTTP server processes
//
// All plugins implement the base Plugin interface. Type-specific interfaces
// (ToolPlugin, ProviderPlugin) extend this for specialized functionality.
package plugin

import "context"

// PluginType classifies what a plugin provides.
type PluginType string

const (
	TypeTool     PluginType = "tool"
	TypeProvider PluginType = "provider"
	TypeAgent    PluginType = "agent"
	TypeSkill    PluginType = "skill"
)

// PluginMode indicates how the plugin is loaded.
type PluginMode string

const (
	ModeLib    PluginMode = "lib"    // .so file loaded via plugin.Open()
	ModeBinary PluginMode = "binary" // Separate HTTP server process
)

// Plugin is the base interface all plugins must implement.
// For lib-mode plugins, export a variable named QuarkPlugin of this type.
type Plugin interface {
	// Name returns the plugin's unique identifier.
	Name() string

	// Version returns the semantic version string.
	Version() string

	// Type returns the plugin type (tool, provider, agent, skill).
	Type() PluginType

	// Initialize is called after loading to set up the plugin.
	// Config contains plugin-specific configuration from the manifest.
	Initialize(ctx context.Context, config map[string]any) error

	// Shutdown is called before unloading to clean up resources.
	Shutdown(ctx context.Context) error
}

// LoadedPlugin represents a successfully loaded plugin instance.
type LoadedPlugin struct {
	Manifest *Manifest  // Parsed manifest
	Plugin   Plugin     // Plugin instance (nil for binary-only mode)
	Mode     PluginMode // How it was loaded
	Dir      string     // Plugin directory path
}
