package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultHubURL is the default plugin hub API endpoint.
const DefaultHubURL = "https://hub.quarkloop.com/api/v1"

// HubClient communicates with the plugin hub API.
type HubClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewHubClient creates a hub client with the given base URL.
func NewHubClient(baseURL string) *HubClient {
	if baseURL == "" {
		baseURL = DefaultHubURL
	}
	return &HubClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// PluginSearchItem is a result from the hub search.
type PluginSearchItem struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Author      string `json:"author"`
}

// Search queries the hub for plugins matching the query.
func (c *HubClient) Search(query string) ([]PluginSearchItem, error) {
	url := fmt.Sprintf("%s/plugins?q=%s", c.BaseURL, query)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("hub search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub search returned %d", resp.StatusCode)
	}

	var result struct {
		Items []PluginSearchItem `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse hub search response: %w", err)
	}
	return result.Items, nil
}

// GetManifest fetches the raw manifest.yaml content for a plugin from the hub.
func (c *HubClient) GetManifest(name, version string) ([]byte, error) {
	if version == "" {
		version = "latest"
	}
	url := fmt.Sprintf("%s/plugins/%s/%s/manifest", c.BaseURL, name, version)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("hub get manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub get manifest returned %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// Manager manages plugins installed on disk.
type Manager struct{}

// NewManager creates a fresh plugin manager.
func NewManager() *Manager {
	return &Manager{}
}

// Install copies a plugin from a local directory into the space's plugin directory.
// It reads the manifest, validates the type, and returns the manifest.
func (m *Manager) Install(srcDir, pluginsDir string) (*Manifest, error) {
	manifestPath := filepath.Join(srcDir, "manifest.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, fmt.Errorf("no manifest.yaml in %s", srcDir)
	}

	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, fmt.Errorf("resolve source: %w", err)
	}

	man, err := LoadLocal(absSrc)
	if err != nil {
		return nil, err
	}

	// Determine the target directory.
	destDir := filepath.Join(pluginsDir, man.TypeDirName())
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("create plugin dir: %w", err)
	}

	// Copy manifest.
	manifestYAML, err := os.ReadFile(filepath.Join(absSrc, "manifest.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "manifest.yaml"), manifestYAML, 0644); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	// Copy SKILL.md if present.
	skillPath := filepath.Join(absSrc, "SKILL.md")
	if data, err := os.ReadFile(skillPath); err == nil {
		if err := os.WriteFile(filepath.Join(destDir, "SKILL.md"), data, 0644); err != nil {
			return nil, fmt.Errorf("write SKILL.md: %w", err)
		}
	}

	// Copy bin/ directory if present (tool plugins).
	binSrc := filepath.Join(absSrc, "bin")
	if stat, err := os.Stat(binSrc); err == nil && stat.IsDir() {
		binDest := filepath.Join(destDir, "bin")
		if err := copyDir(binSrc, binDest); err != nil {
			return nil, fmt.Errorf("copy bin dir: %w", err)
		}
	}

	// Copy prompt file for agent plugins.
	if man.Type == TypeAgent && man.Prompt != "" {
		promptData, err := os.ReadFile(man.Prompt)
		if err != nil {
			return nil, fmt.Errorf("read agent prompt %q: %w", man.Prompt, err)
		}
		promptName := filepath.Base(man.Prompt)
		if err := os.WriteFile(filepath.Join(destDir, promptName), promptData, 0644); err != nil {
			return nil, fmt.Errorf("write agent prompt: %w", err)
		}
	}

	return man, nil
}

// Uninstall removes an installed plugin from disk.
func (m *Manager) Uninstall(pluginsDir, nameOrTypeDir string) error {
	destDir := filepath.Join(pluginsDir, nameOrTypeDir)
	if _, err := os.Stat(destDir); err != nil {
		return fmt.Errorf("plugin %q not found: %w", nameOrTypeDir, err)
	}
	return os.RemoveAll(destDir)
}

// List returns all installed plugin directories with their manifests.
func (m *Manager) List(pluginsDir string) ([]Plugin, error) {
	return DiscoverInstalled(pluginsDir)
}

// Get returns a specific plugin by its {type}-{name} directory name.
func (m *Manager) Get(pluginsDir, name string) (*Plugin, error) {
	plugins, err := m.List(pluginsDir)
	if err != nil {
		return nil, err
	}
	for _, p := range plugins {
		if p.Manifest.Name == name || p.Manifest.TypeDirName() == name {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("plugin %q not found", name)
}

func copyDir(src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		destPath := filepath.Join(dest, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, destPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(destPath, data, e.Type().Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}
