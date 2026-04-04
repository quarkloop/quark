// Package pluginsvc defines the plugin service interface and a local
// implementation that wraps the existing plugin Manager and installer.
package plugin

import (
	"context"
)

// Service defines operations for managing plugins. Plugins are local-only
// (filesystem operations) — no HTTP mode is needed.
type Service interface {
	Install(ctx context.Context, ref, pluginsDir string) (*Manifest, error)
	Uninstall(ctx context.Context, name, pluginsDir string) error
	List(ctx context.Context, pluginsDir string) ([]Plugin, error)
	Info(ctx context.Context, name, pluginsDir string) (*Plugin, error)
	Search(ctx context.Context, query string) ([]PluginSearchItem, error)
	Build(ctx context.Context, dir string) (*Manifest, error)
	Update(ctx context.Context, name, pluginsDir string) (*Manifest, error)
}
