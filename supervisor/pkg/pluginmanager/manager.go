// Package pluginmanager handles plugin installation, uninstallation, and
// discovery within a space's plugins directory. The absolute plugins
// directory is supplied by the caller; the manager has no knowledge of
// the surrounding space layout.
package pluginmanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/quarkloop/pkg/plugin"
)

// Manager handles plugin management for a single plugins directory.
type Manager struct {
	mu         sync.RWMutex
	pluginsDir string
	hubClient  *HubClient
}

// NewManager creates a plugin manager rooted at pluginsDir. The directory
// must be the absolute path to the space's plugins directory.
func NewManager(pluginsDir string) *Manager {
	return &Manager{
		pluginsDir: pluginsDir,
		hubClient:  NewHubClient(""),
	}
}

// PluginsDir returns the absolute plugins directory managed by this Manager.
func (m *Manager) PluginsDir() string {
	return m.pluginsDir
}

// InstalledPlugin represents an installed plugin with its path.
type InstalledPlugin struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Type        plugin.PluginType `json:"type"`
	Mode        plugin.PluginMode `json:"mode"`
	Description string            `json:"description"`
	Path        string            `json:"path"` // Full path to plugin directory
}

// List returns all installed plugins in the space.
func (m *Manager) List() ([]InstalledPlugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var plugins []InstalledPlugin

	pluginsDir := m.PluginsDir()

	// Scan tools/
	toolsDir := filepath.Join(pluginsDir, "tools")
	if entries, err := os.ReadDir(toolsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pluginDir := filepath.Join(toolsDir, e.Name())
			manifest, err := plugin.ParseManifest(filepath.Join(pluginDir, "manifest.yaml"))
			if err != nil {
				continue
			}
			plugins = append(plugins, InstalledPlugin{
				Name:        manifest.Name,
				Version:     manifest.Version,
				Type:        manifest.Type,
				Mode:        manifest.Mode,
				Description: manifest.Description,
				Path:        pluginDir,
			})
		}
	}

	// Scan providers/
	providersDir := filepath.Join(pluginsDir, "providers")
	if entries, err := os.ReadDir(providersDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pluginDir := filepath.Join(providersDir, e.Name())
			manifest, err := plugin.ParseManifest(filepath.Join(pluginDir, "manifest.yaml"))
			if err != nil {
				continue
			}
			plugins = append(plugins, InstalledPlugin{
				Name:        manifest.Name,
				Version:     manifest.Version,
				Type:        manifest.Type,
				Mode:        manifest.Mode,
				Description: manifest.Description,
				Path:        pluginDir,
			})
		}
	}

	// Scan agents/
	agentsDir := filepath.Join(pluginsDir, "agents")
	if entries, err := os.ReadDir(agentsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pluginDir := filepath.Join(agentsDir, e.Name())
			manifest, err := plugin.ParseManifest(filepath.Join(pluginDir, "manifest.yaml"))
			if err != nil {
				continue
			}
			plugins = append(plugins, InstalledPlugin{
				Name:        manifest.Name,
				Version:     manifest.Version,
				Type:        manifest.Type,
				Mode:        manifest.Mode,
				Description: manifest.Description,
				Path:        pluginDir,
			})
		}
	}

	// Scan skills/
	skillsDir := filepath.Join(pluginsDir, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pluginDir := filepath.Join(skillsDir, e.Name())
			manifest, err := plugin.ParseManifest(filepath.Join(pluginDir, "manifest.yaml"))
			if err != nil {
				continue
			}
			plugins = append(plugins, InstalledPlugin{
				Name:        manifest.Name,
				Version:     manifest.Version,
				Type:        manifest.Type,
				Mode:        manifest.Mode,
				Description: manifest.Description,
				Path:        pluginDir,
			})
		}
	}

	return plugins, nil
}

// ListByType returns installed plugins of a specific type.
func (m *Manager) ListByType(pluginType plugin.PluginType) ([]InstalledPlugin, error) {
	all, err := m.List()
	if err != nil {
		return nil, err
	}

	var filtered []InstalledPlugin
	for _, p := range all {
		if p.Type == pluginType {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

// Get returns a specific installed plugin by name.
func (m *Manager) Get(name string) (*InstalledPlugin, error) {
	plugins, err := m.List()
	if err != nil {
		return nil, err
	}

	for _, p := range plugins {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("plugin %q not found", name)
}

// Install installs a plugin from a reference (local path, git URL, or hub name).
func (m *Manager) Install(ctx context.Context, ref string) (*InstalledPlugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pluginsDir := m.PluginsDir()

	// Ensure plugins directory exists
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return nil, fmt.Errorf("create plugins dir: %w", err)
	}

	parsed := ParsePluginRef(ref)

	var srcDir, tmpDir string
	var err error

	switch {
	case IsLocalPath(parsed.Name):
		// Install from local path
		srcDir, err = filepath.Abs(parsed.Name)
		if err != nil {
			return nil, fmt.Errorf("resolve path: %w", err)
		}

	case IsGitURL(parsed.Name):
		// Clone from git
		tmpDir, srcDir, err = cloneSingle(parsed.Name, pluginsDir)
		if err != nil {
			return nil, fmt.Errorf("clone plugin: %w", err)
		}
		defer os.RemoveAll(tmpDir)

	default:
		// Try hub first, then fall back to registry
		info, hubErr := m.hubClient.GetInfo(parsed.Name)
		if hubErr == nil {
			// Install from hub
			version := parsed.Version
			if version == "" {
				version = info.Version
			}
			return m.installFromHub(ctx, parsed.Name, version, pluginsDir)
		}

		// Fall back to registry
		tmpDir, srcDir, err = cloneFromRegistry(registryRoot, parsed.Name, pluginsDir)
		if err != nil {
			return nil, fmt.Errorf("install %q: %w", ref, err)
		}
		defer os.RemoveAll(tmpDir)
	}

	// Parse manifest to determine destination
	manifest, err := plugin.ParseManifest(filepath.Join(srcDir, "manifest.yaml"))
	if err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// Determine destination based on type
	destDir := m.destDirForType(manifest.Type, manifest.Name)
	if _, err := os.Stat(destDir); err == nil {
		return nil, fmt.Errorf("plugin %s already installed", manifest.Name)
	}

	// Copy to destination
	if err := CopyDir(srcDir, destDir); err != nil {
		return nil, fmt.Errorf("copy plugin: %w", err)
	}

	return &InstalledPlugin{
		Name:        manifest.Name,
		Version:     manifest.Version,
		Type:        manifest.Type,
		Mode:        manifest.Mode,
		Description: manifest.Description,
		Path:        destDir,
	}, nil
}

// installFromHub downloads and installs from the plugin hub.
func (m *Manager) installFromHub(ctx context.Context, name, version, pluginsDir string) (*InstalledPlugin, error) {
	// Download from hub
	extractDir, err := m.hubClient.Download(name, version, pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("download from hub: %w", err)
	}
	defer os.RemoveAll(filepath.Dir(extractDir))

	// Parse manifest
	manifest, err := plugin.ParseManifest(filepath.Join(extractDir, "manifest.yaml"))
	if err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// Determine destination
	destDir := m.destDirForType(manifest.Type, manifest.Name)
	if _, err := os.Stat(destDir); err == nil {
		return nil, fmt.Errorf("plugin %s already installed", manifest.Name)
	}

	// Copy to destination
	if err := CopyDir(extractDir, destDir); err != nil {
		return nil, fmt.Errorf("copy plugin: %w", err)
	}

	return &InstalledPlugin{
		Name:        manifest.Name,
		Version:     manifest.Version,
		Type:        manifest.Type,
		Mode:        manifest.Mode,
		Description: manifest.Description,
		Path:        destDir,
	}, nil
}

// destDirForType returns the destination directory for a plugin based on its type.
func (m *Manager) destDirForType(pluginType plugin.PluginType, name string) string {
	pluginsDir := m.PluginsDir()
	switch pluginType {
	case plugin.TypeTool:
		return filepath.Join(pluginsDir, "tools", name)
	case plugin.TypeProvider:
		return filepath.Join(pluginsDir, "providers", name)
	case plugin.TypeAgent:
		return filepath.Join(pluginsDir, "agents", name)
	case plugin.TypeSkill:
		return filepath.Join(pluginsDir, "skills", name)
	default:
		return filepath.Join(pluginsDir, string(pluginType)+"s", name)
	}
}

// Uninstall removes an installed plugin.
func (m *Manager) Uninstall(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the plugin
	plugins, err := m.List()
	if err != nil {
		return err
	}

	for _, p := range plugins {
		if p.Name == name {
			return os.RemoveAll(p.Path)
		}
	}

	return fmt.Errorf("plugin %q not found", name)
}

// Search searches the hub for plugins matching the query.
func (m *Manager) Search(query string) ([]PluginSearchItem, error) {
	return m.hubClient.Search(query)
}

// GetHubInfo returns information about a plugin from the hub.
func (m *Manager) GetHubInfo(name string) (*PluginInfo, error) {
	return m.hubClient.GetInfo(name)
}
