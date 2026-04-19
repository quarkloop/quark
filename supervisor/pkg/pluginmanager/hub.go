// Package pluginmanager handles plugin management for spaces.
package pluginmanager

import (
	"archive/tar"
	"compress/gzip"
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

// PluginInfo contains metadata about a plugin in the hub.
type PluginInfo struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	License     string   `json:"license"`
	Repository  string   `json:"repository"`
	Downloads   int      `json:"downloads"`
	Versions    []string `json:"versions"`
}

// GetInfo fetches detailed information about a plugin from the hub.
func (c *HubClient) GetInfo(name string) (*PluginInfo, error) {
	url := fmt.Sprintf("%s/plugins/%s", c.BaseURL, name)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("hub get info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("plugin %q not found in hub", name)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub get info returned %d", resp.StatusCode)
	}

	var info PluginInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("parse hub info response: %w", err)
	}
	return &info, nil
}

// Download downloads a plugin archive from the hub to a temporary directory.
// Returns the path to the extracted plugin directory.
func (c *HubClient) Download(name, version, destDir string) (string, error) {
	if version == "" {
		version = "latest"
	}

	// Create temp dir for download
	tmpDir, err := os.MkdirTemp(destDir, ".hub-download-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Download the archive
	url := fmt.Sprintf("%s/plugins/%s/%s/download", c.BaseURL, name, version)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("hub download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("plugin %s@%s not found in hub", name, version)
	}
	if resp.StatusCode != http.StatusOK {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("hub download returned %d", resp.StatusCode)
	}

	// Save archive to temp file
	archivePath := filepath.Join(tmpDir, "plugin.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("create archive file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("download archive: %w", err)
	}
	f.Close()

	// Extract the archive
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("create extract dir: %w", err)
	}

	if err := extractTarGz(archivePath, extractDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("extract archive: %w", err)
	}

	return extractDir, nil
}

// extractTarGz extracts a .tar.gz archive to the destination directory.
func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		// Sanitize the path to prevent directory traversal
		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("create dir %s: %w", target, err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent dir: %w", err)
			}

			outFile, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("write file %s: %w", target, err)
			}
			outFile.Close()

			// Set file permissions
			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("chmod %s: %w", target, err)
			}
		}
	}

	return nil
}
