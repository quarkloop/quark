// Package plugin defines the plugin manifest, types, and local discovery.
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
	TypeTool  PluginType = "tool"
	TypeAgent PluginType = "agent"
	TypeSkill PluginType = "skill"
)

// Manifest describes a plugin's identity, contents, and requirements.
type Manifest struct {
	Name        string     `yaml:"name"`
	Version     string     `yaml:"version"`
	Type        PluginType `yaml:"type"`
	Description string     `yaml:"description"`
	Author      string     `yaml:"author"`
	License     string     `yaml:"license"`
	Repository  string     `yaml:"repository,omitempty"`

	// Tool plugin fields
	Interface   InterfaceConfig  `yaml:"interface,omitempty"`
	Permissions PermissionConfig `yaml:"permissions,omitempty"`

	// Agent plugin fields
	Prompt string   `yaml:"prompt,omitempty"`
	Tools  []string `yaml:"tools,omitempty"`
	Skills []string `yaml:"skills,omitempty"`
}

// InterfaceConfig declares how a tool plugin is invoked.
type InterfaceConfig struct {
	Mode     []string `yaml:"mode,omitempty"`
	Commands []string `yaml:"commands,omitempty"`
	Endpoint string   `yaml:"endpoint,omitempty"`
}

// PermissionConfig declares what a tool plugin needs.
type PermissionConfig struct {
	Filesystem []string `yaml:"filesystem,omitempty"`
	Network    bool     `yaml:"network"`
}

// Plugin is a loaded plugin with its manifest and resolved directory.
type Plugin struct {
	Manifest *Manifest
	Dir      string
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
		return nil, fmt.Errorf("manifest missing required field: type")
	}
	switch m.Type {
	case TypeTool, TypeAgent, TypeSkill:
	default:
		return nil, fmt.Errorf("manifest has invalid type: %q (expected tool, agent, or skill)", m.Type)
	}
	return &m, nil
}

// LoadLocal loads a plugin from a local directory.
func LoadLocal(dir string) (*Manifest, error) {
	manifestPath := filepath.Join(dir, "manifest.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, fmt.Errorf("no manifest.yaml in %s", dir)
	}
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve plugin dir: %w", err)
	}
	if m.Type == TypeAgent && m.Prompt != "" && !filepath.IsAbs(m.Prompt) {
		m.Prompt = filepath.Join(absDir, m.Prompt)
		if _, err := os.Stat(m.Prompt); err != nil {
			return nil, fmt.Errorf("agent plugin %q: prompt file %q not found", m.Name, m.Prompt)
		}
	}
	return m, nil
}

// DiscoverInstalled scans dir for installed plugins in {type}-{name}/ subdirectories.
func DiscoverInstalled(dir string) ([]Plugin, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading plugin directory: %w", err)
	}
	var plugins []Plugin
	for _, e := range entries {
		if !e.IsDir() || e.Name() == ".registry" {
			continue
		}
		m, err := LoadLocal(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		plugins = append(plugins, Plugin{Manifest: m, Dir: filepath.Join(dir, e.Name())})
	}
	return plugins, nil
}

// TypeDirName returns the {type}-{name} directory name for a plugin.
func (m *Manifest) TypeDirName() string {
	return string(m.Type) + "-" + nameFromManifest(m)
}

func nameFromManifest(m *Manifest) string {
	name := m.Name
	if strings.HasPrefix(name, "tool-") {
		return strings.TrimPrefix(name, "tool-")
	}
	if strings.HasPrefix(name, "agent-") {
		return strings.TrimPrefix(name, "agent-")
	}
	if strings.HasPrefix(name, "skill-") {
		return strings.TrimPrefix(name, "skill-")
	}
	return name
}
