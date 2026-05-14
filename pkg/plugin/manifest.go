package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Manifest is the plugin declaration loaded from manifest.yaml.
type Manifest struct {
	Name        string     `yaml:"name"`
	Version     string     `yaml:"version"`
	Type        PluginType `yaml:"type"`
	Description string     `yaml:"description"`
	Author      string     `yaml:"author,omitempty"`
	License     string     `yaml:"license,omitempty"`
	Repository  string     `yaml:"repository,omitempty"`

	// Plugin loading mode
	Mode  PluginMode   `yaml:"mode"`
	Build *BuildConfig `yaml:"build,omitempty"`

	// Type-specific nested configs (only one should be set based on Type)
	Tool     *ToolConfig             `yaml:"tool,omitempty"`
	Provider *ProviderManifestConfig `yaml:"provider,omitempty"`
	Agent    *AgentConfig            `yaml:"agent,omitempty"`
	Skill    *SkillConfig            `yaml:"skill,omitempty"`
	Service  *ServiceConfig          `yaml:"service,omitempty"`
}

// BuildConfig holds build-related configuration.
type BuildConfig struct {
	LibTarget     string `yaml:"lib_target,omitempty"`      // Output .so file name
	APITarget     string `yaml:"api_target,omitempty"`      // Output binary name (api mode)
	APIEntryPoint string `yaml:"api_entry_point,omitempty"` // Main package for api mode
}

// AgentConfig holds agent-specific configuration from the manifest.
type AgentConfig struct {
	Prompt string   `yaml:"prompt,omitempty"` // Path to agent prompt file
	Tools  []string `yaml:"tools,omitempty"`  // Required tool plugins
	Skills []string `yaml:"skills,omitempty"` // Required skill plugins
}

// SkillConfig holds skill-specific configuration from the manifest.
type SkillConfig struct {
	// Future: skill-specific config
}

// ServiceConfig declares a gRPC service exposed through the plugin catalog.
type ServiceConfig struct {
	// AddressEnv names the environment variable that contains the service
	// address. The supervisor resolves it before runtime startup.
	AddressEnv string `yaml:"address_env,omitempty"`
	// DefaultAddress is used when AddressEnv is empty or unset.
	DefaultAddress string `yaml:"default_address,omitempty"`
	// Skill names the service skill file relative to the plugin directory.
	Skill string `yaml:"skill,omitempty"`
	// ProtoServices lists protobuf service names exposed by this service.
	ProtoServices []string `yaml:"proto_services,omitempty"`
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

	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest: %w", err)
	}

	return &m, nil
}

// Validate checks that the manifest has all required fields and valid values.
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	if m.Type == "" {
		return fmt.Errorf("type is required")
	}

	// Validate type
	switch m.Type {
	case TypeTool, TypeProvider, TypeAgent, TypeSkill, TypeService:
		// valid
	default:
		return fmt.Errorf("invalid type: %s (must be tool, provider, agent, skill, or service)", m.Type)
	}

	// Validate mode
	if m.Mode == "" {
		m.Mode = ModeAPI // default to api mode for backward compatibility
	}
	switch m.Mode {
	case ModeLib, ModeAPI, ModeCLI:
		// valid
	default:
		return fmt.Errorf("invalid mode: %s (must be lib, api, or cli)", m.Mode)
	}

	// Validate type-specific config is present
	switch m.Type {
	case TypeTool:
		if m.Tool == nil {
			return fmt.Errorf("tool config is required for tool plugins")
		}
		if m.Tool.Schema.Name == "" {
			return fmt.Errorf("tool.schema.name is required")
		}
	case TypeProvider:
		if m.Provider == nil {
			return fmt.Errorf("provider config is required for provider plugins")
		}
		if m.Provider.APIBase == "" {
			return fmt.Errorf("provider.api_base is required")
		}
		if m.Provider.AuthEnv == "" {
			return fmt.Errorf("provider.auth_env is required")
		}
	case TypeAgent:
		if m.Agent == nil {
			return fmt.Errorf("agent config is required for agent plugins")
		}
	case TypeSkill:
		// Skill config is optional
	case TypeService:
		if m.Service == nil {
			return fmt.Errorf("service config is required for service plugins")
		}
		if m.Service.Skill == "" {
			m.Service.Skill = "SKILL.md"
		}
	}

	return nil
}

// LibTargetPath returns the path to the .so file for lib-mode plugins.
func (m *Manifest) LibTargetPath(pluginDir string) string {
	target := "plugin.so"
	if m.Build != nil && m.Build.LibTarget != "" {
		target = m.Build.LibTarget
	}
	return filepath.Join(pluginDir, target)
}

// APITargetPath returns the path to the binary for api-mode plugins.
func (m *Manifest) APITargetPath(binDir string) string {
	target := m.Name
	if m.Build != nil && m.Build.APITarget != "" {
		target = m.Build.APITarget
	}
	return filepath.Join(binDir, target)
}

// APIEntryPointPath returns the main package path for api-mode builds.
func (m *Manifest) APIEntryPointPath() string {
	if m.Build != nil && m.Build.APIEntryPoint != "" {
		return m.Build.APIEntryPoint
	}
	return filepath.Join("cmd", m.Name)
}
