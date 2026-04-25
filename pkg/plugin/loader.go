package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goplugin "plugin"
	"sync"
	"time"
)

// Loader handles plugin discovery and loading for lib, api, and cli modes.
type Loader struct {
	pluginsDir string
	binDir     string // Directory for built binaries

	mu       sync.RWMutex
	libCache map[string]*goplugin.Plugin // Loaded .so files
	apiProcs map[string]*APIProcess      // Running api-mode processes
	nextPort int                         // Next available port for api plugins
}

// APIProcess tracks a running api-mode plugin.
type APIProcess struct {
	Cmd      *exec.Cmd
	Endpoint string // HTTP endpoint (e.g., "http://127.0.0.1:8096")
	Port     int
}

// NewLoader creates a plugin loader.
func NewLoader(pluginsDir string) *Loader {
	return &Loader{
		pluginsDir: pluginsDir,
		binDir:     filepath.Join(pluginsDir, ".bin"),
		libCache:   make(map[string]*goplugin.Plugin),
		apiProcs:   make(map[string]*APIProcess),
		nextPort:   8100, // Start api plugins at port 8100
	}
}

// Discover scans the plugins directory and returns all valid manifests.
// Scans plugins/tools/ and plugins/providers/ subdirectories.
func (l *Loader) Discover() ([]*Manifest, error) {
	var manifests []*Manifest

	// Scan tools/
	toolsDir := filepath.Join(l.pluginsDir, "tools")
	if entries, err := os.ReadDir(toolsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			manifestPath := filepath.Join(toolsDir, e.Name(), "manifest.yaml")
			m, err := ParseManifest(manifestPath)
			if err != nil {
				// Log but continue - don't fail on one bad plugin
				continue
			}
			manifests = append(manifests, m)
		}
	}

	// Scan providers/
	providersDir := filepath.Join(l.pluginsDir, "providers")
	if entries, err := os.ReadDir(providersDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			manifestPath := filepath.Join(providersDir, e.Name(), "manifest.yaml")
			m, err := ParseManifest(manifestPath)
			if err != nil {
				continue
			}
			manifests = append(manifests, m)
		}
	}

	// Scan agents/
	agentsDir := filepath.Join(l.pluginsDir, "agents")
	if entries, err := os.ReadDir(agentsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			manifestPath := filepath.Join(agentsDir, e.Name(), "manifest.yaml")
			m, err := ParseManifest(manifestPath)
			if err != nil {
				continue
			}
			manifests = append(manifests, m)
		}
	}

	// Scan skills/
	skillsDir := filepath.Join(l.pluginsDir, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			manifestPath := filepath.Join(skillsDir, e.Name(), "manifest.yaml")
			m, err := ParseManifest(manifestPath)
			if err != nil {
				continue
			}
			manifests = append(manifests, m)
		}
	}

	return manifests, nil
}

// PluginDir returns the directory path for a plugin based on its type.
func (l *Loader) PluginDir(manifest *Manifest) string {
	switch manifest.Type {
	case TypeTool:
		return filepath.Join(l.pluginsDir, "tools", manifest.Name)
	case TypeProvider:
		return filepath.Join(l.pluginsDir, "providers", manifest.Name)
	case TypeAgent:
		return filepath.Join(l.pluginsDir, "agents", manifest.Name)
	case TypeSkill:
		return filepath.Join(l.pluginsDir, "skills", manifest.Name)
	default:
		return filepath.Join(l.pluginsDir, string(manifest.Type)+"s", manifest.Name)
	}
}

// Load loads a plugin based on its manifest mode setting.
func (l *Loader) Load(ctx context.Context, manifest *Manifest, dir string) (*LoadedPlugin, error) {
	switch manifest.Mode {
	case ModeLib:
		return l.loadLib(ctx, manifest, dir)
	case ModeAPI:
		return l.loadAPI(ctx, manifest, dir)
	case ModeCLI:
		return l.loadCLI(ctx, manifest, dir)
	default:
		return nil, fmt.Errorf("unknown plugin mode: %s", manifest.Mode)
	}
}

// loadLib loads a plugin via Go's plugin system.
func (l *Loader) loadLib(ctx context.Context, manifest *Manifest, dir string) (*LoadedPlugin, error) {
	soPath := manifest.LibTargetPath(dir)

	// Check if .so file exists
	if _, err := os.Stat(soPath); err != nil {
		return nil, fmt.Errorf("plugin .so not found at %s: %w", soPath, err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check cache
	if _, ok := l.libCache[manifest.Name]; ok {
		return nil, fmt.Errorf("plugin %s already loaded", manifest.Name)
	}

	// Load the .so file
	p, err := goplugin.Open(soPath)
	if err != nil {
		return nil, fmt.Errorf("open plugin: %w", err)
	}

	// Look up the exported Plugin symbol
	sym, err := p.Lookup("QuarkPlugin")
	if err != nil {
		// Try factory function as fallback
		factorySym, factoryErr := p.Lookup("NewPlugin")
		if factoryErr != nil {
			return nil, fmt.Errorf("plugin has no QuarkPlugin or NewPlugin export: %w", err)
		}
		factory, ok := factorySym.(func() Plugin)
		if !ok {
			return nil, fmt.Errorf("NewPlugin is not func() Plugin")
		}
		plugin := factory()
		if err := plugin.Initialize(ctx, nil); err != nil {
			return nil, fmt.Errorf("initialize plugin: %w", err)
		}
		l.libCache[manifest.Name] = p
		return &LoadedPlugin{
			Manifest: manifest,
			Plugin:   plugin,
			Mode:     ModeLib,
			Dir:      dir,
		}, nil
	}

	// Handle pointer or value export
	var plugin Plugin
	switch v := sym.(type) {
	case *Plugin:
		plugin = *v
	case Plugin:
		plugin = v
	default:
		return nil, fmt.Errorf("QuarkPlugin is not Plugin type (got %T)", sym)
	}

	if err := plugin.Initialize(ctx, nil); err != nil {
		return nil, fmt.Errorf("initialize plugin: %w", err)
	}

	l.libCache[manifest.Name] = p

	return &LoadedPlugin{
		Manifest: manifest,
		Plugin:   plugin,
		Mode:     ModeLib,
		Dir:      dir,
	}, nil
}

// loadCLI verifies a CLI-mode plugin binary exists.
// CLI plugins are invoked as subprocess per call; no long-running process.
func (l *Loader) loadCLI(ctx context.Context, manifest *Manifest, dir string) (*LoadedPlugin, error) {
	// Ensure bin directory exists
	if err := os.MkdirAll(l.binDir, 0755); err != nil {
		return nil, fmt.Errorf("create bin dir: %w", err)
	}

	binPath := manifest.APITargetPath(l.binDir)

	// Check if binary needs to be built
	if _, err := os.Stat(binPath); err != nil {
		entryPoint := manifest.APIEntryPointPath()
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./"+entryPoint)
		buildCmd.Dir = dir
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return nil, fmt.Errorf("build plugin: %w", err)
		}
	}

	return &LoadedPlugin{
		Manifest: manifest,
		Plugin:   nil,
		Mode:     ModeCLI,
		Dir:      dir,
	}, nil
}

// loadAPI builds (if needed) and starts an api-mode plugin.
func (l *Loader) loadAPI(ctx context.Context, manifest *Manifest, dir string) (*LoadedPlugin, error) {
	// Ensure bin directory exists
	if err := os.MkdirAll(l.binDir, 0755); err != nil {
		return nil, fmt.Errorf("create bin dir: %w", err)
	}

	binPath := manifest.APITargetPath(l.binDir)

	// Check if binary needs to be built
	if _, err := os.Stat(binPath); err != nil {
		// Build the binary
		entryPoint := manifest.APIEntryPointPath()
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./"+entryPoint)
		buildCmd.Dir = dir
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return nil, fmt.Errorf("build plugin: %w", err)
		}
	}

	l.mu.Lock()
	port := l.nextPort
	l.nextPort++
	l.mu.Unlock()

	// Start the daemon process
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	daemonCmd := exec.Command(binPath, "serve", "--addr", addr)
	daemonCmd.Stdout = os.Stdout
	daemonCmd.Stderr = os.Stderr

	if err := daemonCmd.Start(); err != nil {
		return nil, fmt.Errorf("start plugin daemon: %w", err)
	}

	// Wait a moment for the server to start
	time.Sleep(100 * time.Millisecond)

	l.mu.Lock()
	l.apiProcs[manifest.Name] = &APIProcess{
		Cmd:      daemonCmd,
		Endpoint: fmt.Sprintf("http://%s", addr),
		Port:     port,
	}
	l.mu.Unlock()

	return &LoadedPlugin{
		Manifest: manifest,
		Plugin:   nil, // API mode has no in-process Plugin
		Mode:     ModeAPI,
		Dir:      dir,
	}, nil
}

// GetAPIEndpoint returns the HTTP endpoint for an api-mode plugin.
func (l *Loader) GetAPIEndpoint(name string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	proc, ok := l.apiProcs[name]
	if !ok {
		return "", false
	}
	return proc.Endpoint, true
}

// Unload stops and unloads a plugin.
func (l *Loader) Unload(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Stop api process if running
	if proc, ok := l.apiProcs[name]; ok {
		if proc.Cmd != nil && proc.Cmd.Process != nil {
			proc.Cmd.Process.Kill()
			proc.Cmd.Wait()
		}
		delete(l.apiProcs, name)
	}

	// Remove from lib cache (Go doesn't support unloading plugins, but we track it)
	delete(l.libCache, name)

	return nil
}

// ShutdownAll stops all running plugin processes.
func (l *Loader) ShutdownAll() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for name, proc := range l.apiProcs {
		if proc.Cmd != nil && proc.Cmd.Process != nil {
			proc.Cmd.Process.Kill()
			proc.Cmd.Wait()
		}
		delete(l.apiProcs, name)
	}
}

// IsLoaded checks if a plugin is currently loaded.
func (l *Loader) IsLoaded(name string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, libOK := l.libCache[name]
	_, apiOK := l.apiProcs[name]
	return libOK || apiOK
}
