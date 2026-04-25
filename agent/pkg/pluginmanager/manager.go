// Package pluginmanager loads and unloads plugins from paths on disk.
//
// The agent does not install or uninstall plugins itself — those operations
// are the supervisor's responsibility since plugins live inside the space
// directory. The agent simply loads plugins that are present in
// .quark/plugins/, executes tools, and dispatches provider calls.
package pluginmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/quarkloop/agent/pkg/provider"
	"github.com/quarkloop/pkg/plugin"
)

// Manager loads tool and provider plugins from disk and dispatches calls to them.
//
// Tool plugins run as api-mode HTTP servers or as lib-mode .so files loaded
// in-process. Provider plugins are always lib-mode (.so) and are wrapped with
// ProviderAdapter so the agent's LLM code can call them through the
// provider.Provider interface.
type Manager struct {
	mu         sync.RWMutex
	pluginsDir string
	binDir     string
	loader     *plugin.Loader

	// API-mode tool plugins
	processes  map[string]*exec.Cmd
	endpoints  map[string]string
	httpClient *http.Client
	nextPort   int

	// Lib-mode tool plugins (.so)
	libTools map[string]plugin.ToolPlugin

	// Tool schemas aggregated from all loaded tools
	tools []provider.Tool

	// Provider plugins (lib mode, wrapped with adapter)
	providers map[string]provider.Provider
}

// NewManager creates a plugin manager rooted at the given plugins directory
// (typically <space>/.quark/plugins).
func NewManager(pluginsDir string) *Manager {
	binDir := filepath.Join(pluginsDir, ".bin")
	_ = os.MkdirAll(binDir, 0755)

	return &Manager{
		pluginsDir: pluginsDir,
		binDir:     binDir,
		loader:     plugin.NewLoader(pluginsDir),
		processes:  make(map[string]*exec.Cmd),
		endpoints:  make(map[string]string),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		nextPort:   8100,
		libTools:   make(map[string]plugin.ToolPlugin),
		providers:  make(map[string]provider.Provider),
		tools:      make([]provider.Tool, 0),
	}
}

// Initialize discovers and loads every tool and provider plugin under
// pluginsDir. Tool plugins in plugins/tools/ are loaded (lib first, binary
// fallback); provider plugins in plugins/providers/ are loaded as .so files
// and configured from their auth_env.
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.loadToolsLocked(ctx); err != nil {
		return fmt.Errorf("load tools: %w", err)
	}
	if err := m.loadProvidersLocked(ctx); err != nil {
		return fmt.Errorf("load providers: %w", err)
	}
	return nil
}

// LoadProviders (re)loads provider plugins from disk. Most callers should use
// Initialize, which already calls this; this method exists for callers that
// need to refresh providers after install/uninstall.
func (m *Manager) LoadProviders(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadProvidersLocked(ctx)
}

// LoadPluginFromDir loads a single plugin from the given directory. Intended
// for use when the supervisor notifies the agent that a new plugin has been
// installed.
func (m *Manager) LoadPluginFromDir(ctx context.Context, dir string) error {
	manifest, err := plugin.ParseManifest(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	switch manifest.Type {
	case plugin.TypeTool:
		return m.loadToolLocked(ctx, manifest, dir)
	case plugin.TypeProvider:
		return m.loadProviderLocked(ctx, manifest, dir)
	default:
		return nil
	}
}

// UnloadPlugin stops and removes a tool plugin by name.
// Returns true if the plugin was loaded and has been unloaded.
func (m *Manager) UnloadPlugin(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	unloaded := false

	if _, ok := m.libTools[name]; ok {
		delete(m.libTools, name)
		unloaded = true
	}

	if cmd, ok := m.processes[name]; ok {
		m.stopProcess(cmd)
		delete(m.processes, name)
		delete(m.endpoints, name)
		unloaded = true
	}

	if unloaded {
		m.removeToolSchema(name)
	}
	return unloaded
}

// GetTools returns the aggregated tool schemas for all loaded tools.
func (m *Manager) GetTools() []provider.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]provider.Tool, len(m.tools))
	copy(out, m.tools)
	return out
}

// GetProviders returns a copy of the loaded provider map keyed by provider ID.
func (m *Manager) GetProviders() map[string]provider.Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]provider.Provider, len(m.providers))
	for k, v := range m.providers {
		out[k] = v
	}
	return out
}

// GetProvider returns a single provider by ID.
func (m *Manager) GetProvider(id string) (provider.Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[id]
	return p, ok
}

// ListLoaded returns the names of all currently-loaded tool plugins.
func (m *Manager) ListLoaded() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.libTools)+len(m.processes))
	for n := range m.libTools {
		names = append(names, n)
	}
	for n := range m.processes {
		names = append(names, n)
	}
	return names
}

// IsLoaded reports whether a tool plugin with the given name is loaded.
func (m *Manager) IsLoaded(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.libTools[name]; ok {
		return true
	}
	_, ok := m.processes[name]
	return ok
}

// ExecuteTool invokes a loaded tool. Lib-mode tools are called in-process;
// api-mode tools are invoked over HTTP.
func (m *Manager) ExecuteTool(ctx context.Context, name, arguments string) (string, error) {
	m.mu.RLock()
	libTool, isLib := m.libTools[name]
	endpoint, isBinary := m.endpoints[name]
	m.mu.RUnlock()

	if isLib {
		return m.executeLib(ctx, libTool, arguments)
	}
	if isBinary {
		return m.executeBinary(ctx, name, endpoint, arguments)
	}
	return "", fmt.Errorf("tool %q not found or not running", name)
}

// Shutdown stops all api-mode plugin processes.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cmd := range m.processes {
		m.stopProcess(cmd)
	}
	m.processes = make(map[string]*exec.Cmd)
	m.endpoints = make(map[string]string)
}

// --- internal helpers (must be called with m.mu held where noted) ---

func (m *Manager) loadToolsLocked(ctx context.Context) error {
	toolsDir := filepath.Join(m.pluginsDir, "tools")
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pluginDir := filepath.Join(toolsDir, e.Name())
		manifest, err := plugin.ParseManifest(filepath.Join(pluginDir, "manifest.yaml"))
		if err != nil {
			fmt.Printf("pluginmanager: skip %s: %v\n", e.Name(), err)
			continue
		}
		if manifest.Type != plugin.TypeTool {
			continue
		}
		if err := m.loadToolLocked(ctx, manifest, pluginDir); err != nil {
			fmt.Printf("pluginmanager: failed to load tool %s: %v\n", manifest.Name, err)
		}
	}
	return nil
}

func (m *Manager) loadProvidersLocked(ctx context.Context) error {
	providersDir := filepath.Join(m.pluginsDir, "providers")
	entries, err := os.ReadDir(providersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pluginDir := filepath.Join(providersDir, e.Name())
		manifest, err := plugin.ParseManifest(filepath.Join(pluginDir, "manifest.yaml"))
		if err != nil {
			fmt.Printf("pluginmanager: skip provider %s: %v\n", e.Name(), err)
			continue
		}
		if manifest.Type != plugin.TypeProvider {
			continue
		}
		if err := m.loadProviderLocked(ctx, manifest, pluginDir); err != nil {
			fmt.Printf("pluginmanager: failed to load provider %s: %v\n", manifest.Name, err)
		}
	}
	return nil
}

func (m *Manager) loadToolLocked(ctx context.Context, manifest *plugin.Manifest, pluginDir string) error {
	if manifest.Mode == plugin.ModeLib {
		if err := m.loadToolLibLocked(ctx, manifest, pluginDir); err == nil {
			return nil
		} else {
			fmt.Printf("pluginmanager: lib mode failed for %s (%v), trying api mode\n", manifest.Name, err)
		}
	}
	return m.loadToolBinaryLocked(ctx, manifest, pluginDir)
}

func (m *Manager) loadToolLibLocked(ctx context.Context, manifest *plugin.Manifest, pluginDir string) error {
	loaded, err := m.loader.Load(ctx, manifest, pluginDir)
	if err != nil {
		return fmt.Errorf("load .so: %w", err)
	}
	if loaded.Plugin == nil {
		return fmt.Errorf("no plugin instance in .so")
	}
	tp, ok := loaded.Plugin.(plugin.ToolPlugin)
	if !ok {
		return fmt.Errorf("%s does not implement ToolPlugin", manifest.Name)
	}

	schema := tp.Schema()
	toolName := schema.Name
	if toolName == "" {
		toolName = manifest.Name
	}

	m.libTools[toolName] = tp
	m.tools = append(m.tools, provider.Tool{
		Type: "function",
		Function: provider.ToolFunction{
			Name:        schema.Name,
			Description: schema.Description,
			Parameters:  schema.Parameters,
		},
	})
	fmt.Printf("pluginmanager: loaded tool %s (lib)\n", toolName)
	return nil
}

func (m *Manager) loadToolBinaryLocked(ctx context.Context, manifest *plugin.Manifest, pluginDir string) error {
	binName := manifest.Name
	if manifest.Build != nil && manifest.Build.APITarget != "" {
		binName = manifest.Build.APITarget
	}
	outPath := filepath.Join(m.binDir, binName)

	// Prefer a pre-built binary shipped alongside the manifest. Installers
	// (including `make build-tools` + plugin install) drop the compiled
	// binary at <pluginDir>/<api_target>; when present, we skip the
	// in-process `go build` step entirely.
	prebuilt := filepath.Join(pluginDir, binName)
	if info, err := os.Stat(prebuilt); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
		outPath = prebuilt
	} else {
		cmdDir := filepath.Join(pluginDir, "cmd", manifest.Name)
		if _, err := os.Stat(cmdDir); os.IsNotExist(err) {
			entries, err := os.ReadDir(filepath.Join(pluginDir, "cmd"))
			if err != nil || len(entries) == 0 {
				return fmt.Errorf("no cmd/ directory found")
			}
			cmdDir = filepath.Join(pluginDir, "cmd", entries[0].Name())
		}

		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", outPath, ".")
		buildCmd.Dir = cmdDir
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("build: %w", err)
		}
	}

	if manifest.Tool != nil {
		m.tools = append(m.tools, provider.Tool{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        manifest.Tool.Schema.Name,
				Description: manifest.Tool.Schema.Description,
				Parameters:  manifest.Tool.Schema.Parameters,
			},
		})
	}

	port := m.nextPort
	m.nextPort++
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	daemonCmd := exec.Command(outPath, "serve", "--addr", addr)
	daemonCmd.Stdout = os.Stdout
	daemonCmd.Stderr = os.Stderr
	if err := daemonCmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	toolName := manifest.Name
	if manifest.Tool != nil && manifest.Tool.Schema.Name != "" {
		toolName = manifest.Tool.Schema.Name
	}

	m.processes[toolName] = daemonCmd
	m.endpoints[toolName] = fmt.Sprintf("http://%s/%s", addr, toolName)

	time.Sleep(50 * time.Millisecond)

	fmt.Printf("pluginmanager: loaded tool %s (binary, %s)\n", toolName, addr)
	return nil
}

func (m *Manager) loadProviderLocked(ctx context.Context, manifest *plugin.Manifest, pluginDir string) error {
	loaded, err := m.loader.Load(ctx, manifest, pluginDir)
	if err != nil {
		return fmt.Errorf("load .so: %w", err)
	}
	if loaded.Plugin == nil {
		return fmt.Errorf("no plugin instance in .so")
	}
	pp, ok := loaded.Plugin.(plugin.ProviderPlugin)
	if !ok {
		return fmt.Errorf("%s does not implement ProviderPlugin", manifest.Name)
	}

	if manifest.Provider != nil && manifest.Provider.AuthEnv != "" {
		apiKey := os.Getenv(manifest.Provider.AuthEnv)
		if apiKey != "" {
			cfg := plugin.ProviderConfig{APIKey: apiKey}
			if manifest.Provider.APIBase != "" {
				cfg.BaseURL = manifest.Provider.APIBase
			}
			if err := pp.Configure(cfg); err != nil {
				return fmt.Errorf("configure provider: %w", err)
			}
		}
	}

	m.providers[pp.ProviderID()] = NewProviderAdapter(pp)
	fmt.Printf("pluginmanager: loaded provider %s (id: %s)\n", manifest.Name, pp.ProviderID())
	return nil
}

func (m *Manager) executeLib(ctx context.Context, tool plugin.ToolPlugin, arguments string) (string, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	result, err := tool.Execute(ctx, args)
	if err != nil {
		return "", fmt.Errorf("execute tool: %w", err)
	}
	if result.IsError {
		resp := map[string]any{"error": result.Error, "is_error": true}
		data, _ := json.Marshal(resp)
		return string(data), nil
	}
	data, err := json.Marshal(result.Output)
	if err != nil {
		return "", fmt.Errorf("serialize result: %w", err)
	}
	return string(data), nil
}

func (m *Manager) executeBinary(ctx context.Context, name, endpoint, arguments string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(arguments))
	if err != nil {
		return "", fmt.Errorf("create plugin request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("plugin http error: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read plugin response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("plugin %q returned HTTP %d: %s", name, resp.StatusCode, string(data))
	}
	return string(data), nil
}

func (m *Manager) removeToolSchema(name string) {
	for i, t := range m.tools {
		if t.Function.Name == name {
			m.tools = append(m.tools[:i], m.tools[i+1:]...)
			return
		}
	}
}

func (m *Manager) stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}
