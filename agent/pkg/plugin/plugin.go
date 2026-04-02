// Package plugin provides the plugin manifest, loader, and manager for
// extending agent capabilities with distributable capability bundles.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginType classifies what a plugin provides.
type PluginType string

const (
	TypeKnowledge PluginType = "knowledge"
	TypeTool      PluginType = "tool"
)

// Manifest describes a plugin's contents and requirements.
type Manifest struct {
	Name        string     `yaml:"name"`
	Version     string     `yaml:"version"`
	Type        PluginType `yaml:"type"`
	Description string     `yaml:"description"`
	Author      string     `yaml:"author"`
	Homepage    string     `yaml:"homepage"`
	License     string     `yaml:"license"`
	Price       string     `yaml:"price"`

	Binaries     []BinaryEntry    `yaml:"binaries,omitempty"`
	Prompts      PromptConfig     `yaml:"prompts,omitempty"`
	KB           []KBEntry        `yaml:"kb,omitempty"`
	Tools        []ToolEntry      `yaml:"tools,omitempty"`
	Dependencies DependencyConfig `yaml:"dependencies,omitempty"`
	Permissions  PermissionConfig `yaml:"permissions"`
	Requires     RequiresConfig   `yaml:"requires"`
	Signature    string           `yaml:"signature"`
}

// BinaryEntry describes a platform-specific binary.
type BinaryEntry struct {
	Platform string `yaml:"platform"`
	Arch     string `yaml:"arch"`
	URL      string `yaml:"url"`
	SHA256   string `yaml:"sha256"`
}

// PromptConfig describes prompt content for a plugin.
type PromptConfig struct {
	System string           `yaml:"system"`
	Skills []SkillReference `yaml:"skills,omitempty"`
}

// SkillReference points to a skill file within the plugin.
type SkillReference struct {
	Name        string `yaml:"name"`
	Path        string `yaml:"path"`
	Description string `yaml:"description"`
}

// KBEntry describes a KB file to load.
type KBEntry struct {
	Path      string `yaml:"path"`
	Namespace string `yaml:"namespace"`
}

// ToolEntry describes a tool provided by the plugin.
type ToolEntry struct {
	Name         string `yaml:"name"`
	Endpoint     string `yaml:"endpoint"`
	InputSchema  string `yaml:"input_schema"`
	OutputSchema string `yaml:"output_schema"`
}

// PermissionConfig declares what the plugin needs.
type PermissionConfig struct {
	Filesystem FilesystemPermConfig `yaml:"filesystem"`
	Network    NetworkPermConfig    `yaml:"network"`
	Tools      ToolPermConfig       `yaml:"tools"`
}

// FilesystemPermConfig describes filesystem permissions.
type FilesystemPermConfig struct {
	Read  []string `yaml:"read"`
	Write []string `yaml:"write"`
}

// NetworkPermConfig describes network permissions.
type NetworkPermConfig struct {
	AllowedHosts []string `yaml:"allowed_hosts"`
}

// ToolPermConfig describes tool permissions.
type ToolPermConfig struct {
	CanCall []string `yaml:"can_call"`
}

// DependencyConfig lists plugin and tool dependencies.
type DependencyConfig struct {
	Plugins []string `yaml:"plugins"`
	Tools   []string `yaml:"tools"`
}

// RequiresConfig lists minimum version requirements.
type RequiresConfig struct {
	QuarkVersion string `yaml:"quark_version"`
}

// Plugin is a loaded plugin with its manifest and resolved paths.
type Plugin struct {
	Manifest *Manifest
	Dir      string // root directory where the plugin is stored
}

// ParseManifest reads and parses a manifest.yaml file.
func ParseManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if m.Name == "" {
		return nil, fmt.Errorf("manifest missing required field: name")
	}
	if m.Version == "" {
		return nil, fmt.Errorf("manifest missing required field: version")
	}
	if m.Type == "" {
		m.Type = TypeKnowledge
	}

	return &m, nil
}

// LoadLocal loads a plugin from a local directory.
func LoadLocal(dir string) (*Plugin, error) {
	manifestPath := filepath.Join(dir, "manifest.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, fmt.Errorf("no manifest.yaml in %s", dir)
	}

	m, err := ParseManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	// Resolve relative paths in the manifest to absolute paths.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve plugin dir: %w", err)
	}

	// Resolve prompt paths.
	if m.Prompts.System != "" && !filepath.IsAbs(m.Prompts.System) {
		m.Prompts.System = filepath.Join(absDir, m.Prompts.System)
	}
	for i := range m.Prompts.Skills {
		if !filepath.IsAbs(m.Prompts.Skills[i].Path) {
			m.Prompts.Skills[i].Path = filepath.Join(absDir, m.Prompts.Skills[i].Path)
		}
	}

	// Resolve KB paths.
	for i := range m.KB {
		if !filepath.IsAbs(m.KB[i].Path) {
			m.KB[i].Path = filepath.Join(absDir, m.KB[i].Path)
		}
	}

	// Resolve tool schema paths.
	for i := range m.Tools {
		if m.Tools[i].InputSchema != "" && !filepath.IsAbs(m.Tools[i].InputSchema) {
			m.Tools[i].InputSchema = filepath.Join(absDir, m.Tools[i].InputSchema)
		}
		if m.Tools[i].OutputSchema != "" && !filepath.IsAbs(m.Tools[i].OutputSchema) {
			m.Tools[i].OutputSchema = filepath.Join(absDir, m.Tools[i].OutputSchema)
		}
	}

	return &Plugin{
		Manifest: m,
		Dir:      absDir,
	}, nil
}

// PluginStatus tracks a loaded plugin's lifecycle state.
type PluginStatus string

const (
	StatusLoading  PluginStatus = "loading"
	StatusActive   PluginStatus = "active"
	StatusFailed   PluginStatus = "failed"
	StatusUnloaded PluginStatus = "unloaded"
)

// LoadedPlugin is a plugin that has been loaded and is being managed.
type LoadedPlugin struct {
	Manifest       *Manifest
	Dir            string
	EffectivePerms *PermissionConfig
	Status         PluginStatus
}

// Manager handles plugin lifecycle — loading, permission intersection,
// content merging, and binary start/stop.
type Manager struct {
	plugins map[string]*LoadedPlugin
}

// NewManager creates a plugin manager.
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]*LoadedPlugin),
	}
}

// Install loads a plugin from a local directory, verifies permissions,
// and registers it with the manager.
func (m *Manager) Install(dir string, quarkfilePerms *QuarkfilePerms) (*LoadedPlugin, error) {
	p, err := LoadLocal(dir)
	if err != nil {
		return nil, fmt.Errorf("load plugin: %w", err)
	}

	loaded := &LoadedPlugin{
		Manifest:       p.Manifest,
		Dir:            p.Dir,
		EffectivePerms: IntersectPermissions(&p.Manifest.Permissions, quarkfilePerms),
		Status:         StatusActive,
	}

	m.plugins[p.Manifest.Name] = loaded
	return loaded, nil
}

// Uninstall removes a plugin from the manager.
func (m *Manager) Uninstall(name string) error {
	if _, ok := m.plugins[name]; !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	delete(m.plugins, name)
	return nil
}

// List returns all loaded plugins.
func (m *Manager) List() []*LoadedPlugin {
	result := make([]*LoadedPlugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result
}

// Get returns a specific loaded plugin.
func (m *Manager) Get(name string) (*LoadedPlugin, error) {
	p, ok := m.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}
	return p, nil
}

// QuarkfilePerms mirrors the Quarkfile permissions block for intersection.
type QuarkfilePerms struct {
	FilesystemAllowedPaths []string
	FilesystemReadOnly     []string
	NetworkAllowedHosts    []string
	ToolsAllowed           []string
}

// IntersectPermissions computes effective permissions as the intersection
// of what the plugin requests and what the Quarkfile allows.
func IntersectPermissions(manifest *PermissionConfig, quarkfile *QuarkfilePerms) *PermissionConfig {
	if manifest == nil || quarkfile == nil {
		return &PermissionConfig{}
	}

	effective := &PermissionConfig{}

	// Filesystem: intersection of manifest paths and Quarkfile allowed paths.
	effective.Filesystem.Read = intersectPaths(manifest.Filesystem.Read, quarkfile.FilesystemAllowedPaths)
	effective.Filesystem.Write = intersectPaths(manifest.Filesystem.Write, quarkfile.FilesystemAllowedPaths)

	// Network: intersection of manifest hosts and Quarkfile allowed hosts.
	effective.Network.AllowedHosts = intersectStrings(manifest.Network.AllowedHosts, quarkfile.NetworkAllowedHosts)

	// Tools: intersection of manifest tools and Quarkfile allowed tools.
	effective.Tools.CanCall = intersectStrings(manifest.Tools.CanCall, quarkfile.ToolsAllowed)

	return effective
}

func intersectPaths(manifestPaths, allowedPaths []string) []string {
	if len(allowedPaths) == 0 {
		return manifestPaths
	}
	var result []string
	for _, mp := range manifestPaths {
		for _, ap := range allowedPaths {
			if strings.HasPrefix(mp, ap) || mp == ap {
				result = append(result, mp)
				break
			}
		}
	}
	return result
}

func intersectStrings(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	set := make(map[string]bool, len(b))
	for _, s := range b {
		set[s] = true
	}
	var result []string
	for _, s := range a {
		if set[s] {
			result = append(result, s)
		}
	}
	return result
}
