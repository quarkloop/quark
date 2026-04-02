package plugin

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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

// PluginInfo is the metadata returned by the hub.
type PluginInfo struct {
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Author      string  `json:"author"`
	Downloads   int     `json:"downloads"`
	Rating      float64 `json:"rating"`
}

// Search queries the hub for plugins matching the query.
func (c *HubClient) Search(query string) ([]PluginInfo, error) {
	url := fmt.Sprintf("%s/plugins?q=%s", c.BaseURL, query)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("hub search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub search returned %d", resp.StatusCode)
	}

	// Parse response — simplified for now.
	return nil, nil
}

// GetPlugin fetches plugin metadata from the hub.
func (c *HubClient) GetPlugin(name, version string) (*Manifest, error) {
	url := fmt.Sprintf("%s/plugins/%s/%s", c.BaseURL, name, version)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("hub get plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub get plugin returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read hub response: %w", err)
	}

	var m Manifest
	if err := unmarshalJSON(data, &m); err != nil {
		return nil, fmt.Errorf("parse hub manifest: %w", err)
	}

	return &m, nil
}

// DownloadBinary downloads a plugin binary from the hub.
func (c *HubClient) DownloadBinary(name, version, platform, arch string) ([]byte, error) {
	url := fmt.Sprintf("%s/plugins/%s/%s/download/%s/%s", c.BaseURL, name, version, platform, arch)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("hub download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub download returned %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// DownloadContent downloads the plugin content archive (prompts, skills, KB).
func (c *HubClient) DownloadContent(name, version string) ([]byte, error) {
	url := fmt.Sprintf("%s/plugins/%s/%s/content", c.BaseURL, name, version)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("hub download content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub download content returned %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// VerifyBinary verifies a plugin binary's signature and checksum.
func VerifyBinary(binary []byte, expectedSHA256, publisherSigHex, pubKeyHex string) error {
	// Verify SHA-256 checksum.
	actualSHA256 := sha256Hex(binary)
	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf("binary checksum mismatch: expected %s, got %s", expectedSHA256, actualSHA256)
	}

	// Verify publisher signature.
	if pubKeyHex != "" && publisherSigHex != "" {
		pubKey, err := hex.DecodeString(pubKeyHex)
		if err != nil {
			return fmt.Errorf("decode publisher public key: %w", err)
		}
		sig, err := hex.DecodeString(publisherSigHex)
		if err != nil {
			return fmt.Errorf("decode publisher signature: %w", err)
		}
		if len(pubKey) != ed25519.PublicKeySize {
			return fmt.Errorf("invalid publisher public key size: %d", len(pubKey))
		}
		if !ed25519.Verify(ed25519.PublicKey(pubKey), binary, sig) {
			return fmt.Errorf("publisher signature verification failed")
		}
	}

	return nil
}

// InstallFromHub downloads, verifies, and installs a plugin from the hub.
func (m *Manager) InstallFromHub(name, version string, hubClient *HubClient, quarkfilePerms *QuarkfilePerms) (*LoadedPlugin, error) {
	if hubClient == nil {
		hubClient = NewHubClient("")
	}

	if version == "" {
		version = "latest"
	}

	// Fetch plugin metadata from hub.
	manifest, err := hubClient.GetPlugin(name, version)
	if err != nil {
		return nil, fmt.Errorf("get plugin from hub: %w", err)
	}

	// Create plugin directory.
	pluginDir := filepath.Join(".quark", "plugins", name, version)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return nil, fmt.Errorf("create plugin dir: %w", err)
	}

	// Download and verify binary if this is a Tool Pack.
	if manifest.Type == TypeTool {
		platform := runtime.GOOS
		arch := runtime.GOARCH
		binaryData, err := hubClient.DownloadBinary(name, version, platform, arch)
		if err != nil {
			return nil, fmt.Errorf("download binary: %w", err)
		}

		// Find matching binary entry for checksum.
		var expectedSHA256 string
		for _, b := range manifest.Binaries {
			if b.Platform == platform && b.Arch == arch {
				expectedSHA256 = b.SHA256
				break
			}
		}

		binaryPath := filepath.Join(pluginDir, "bin", platform+"-"+arch)
		if err := os.MkdirAll(filepath.Dir(binaryPath), 0755); err != nil {
			return nil, fmt.Errorf("create bin dir: %w", err)
		}
		if err := os.WriteFile(binaryPath, binaryData, 0755); err != nil {
			return nil, fmt.Errorf("write binary: %w", err)
		}

		// Verify binary.
		if err := VerifyBinary(binaryData, expectedSHA256, manifest.Signature, ""); err != nil {
			os.RemoveAll(pluginDir)
			return nil, fmt.Errorf("verify binary: %w", err)
		}
	}

	// Download content archive.
	contentData, err := hubClient.DownloadContent(name, version)
	if err != nil {
		return nil, fmt.Errorf("download content: %w", err)
	}

	// Write manifest.
	manifestData, err := marshalYAML(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), manifestData, 0644); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	// Extract content (simplified — in production, use tar extraction).
	if len(contentData) > 0 {
		contentDir := filepath.Join(pluginDir, "content")
		if err := os.MkdirAll(contentDir, 0755); err != nil {
			return nil, fmt.Errorf("create content dir: %w", err)
		}
		// Write content as-is for now.
		if err := os.WriteFile(filepath.Join(contentDir, "archive"), contentData, 0644); err != nil {
			return nil, fmt.Errorf("write content: %w", err)
		}
	}

	// Load the plugin from the local directory.
	return m.Install(pluginDir, quarkfilePerms)
}

// sha256Hex computes the SHA-256 hash of data and returns it as a hex string.
func sha256Hex(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// unmarshalJSON is a JSON unmarshaler.
func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// marshalYAML marshals a value to YAML.
func marshalYAML(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}
