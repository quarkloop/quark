package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ToolRegistry manages loaded tool plugins.
type ToolRegistry struct {
	mu     sync.RWMutex
	tools  map[string]*LoadedTool
	loader *Loader
	client *http.Client
}

// LoadedTool represents a registered tool with its execution context.
type LoadedTool struct {
	*LoadedPlugin
	ToolPlugin ToolPlugin // For lib mode (nil for binary mode)
	Endpoint   string     // For binary mode HTTP endpoint
}

// NewToolRegistry creates a new tool registry.
func NewToolRegistry(loader *Loader) *ToolRegistry {
	return &ToolRegistry{
		tools:  make(map[string]*LoadedTool),
		loader: loader,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Register adds a loaded plugin to the tool registry.
func (r *ToolRegistry) Register(loaded *LoadedPlugin) error {
	if loaded.Manifest.Type != TypeTool {
		return fmt.Errorf("plugin %s is not a tool plugin", loaded.Manifest.Name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	lt := &LoadedTool{LoadedPlugin: loaded}

	if loaded.Mode == ModeLib {
		tp, ok := loaded.Plugin.(ToolPlugin)
		if !ok {
			return fmt.Errorf("plugin %s does not implement ToolPlugin", loaded.Manifest.Name)
		}
		lt.ToolPlugin = tp
	} else {
		// Binary mode - get endpoint from loader
		endpoint, ok := r.loader.GetBinaryEndpoint(loaded.Manifest.Name)
		if !ok {
			return fmt.Errorf("no endpoint found for binary plugin %s", loaded.Manifest.Name)
		}
		// Use the tool's schema name as the endpoint path
		lt.Endpoint = endpoint + "/" + loaded.Manifest.Tool.Schema.Name
	}

	r.tools[loaded.Manifest.Name] = lt
	return nil
}

// Unregister removes a tool from the registry.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Get returns a loaded tool by name.
func (r *ToolRegistry) Get(name string) (*LoadedTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// Execute runs a tool by name with the given arguments.
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]any) (ToolResult, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return ToolResult{IsError: true, Error: fmt.Sprintf("tool %q not found", name)}, nil
	}

	if tool.Mode == ModeLib {
		return tool.ToolPlugin.Execute(ctx, args)
	}

	// HTTP call for binary mode
	return r.httpExecute(ctx, tool.Endpoint, args)
}

// httpExecute sends an HTTP POST request to a binary-mode tool.
func (r *ToolRegistry) httpExecute(ctx context.Context, endpoint string, args map[string]any) (ToolResult, error) {
	body, err := json.Marshal(args)
	if err != nil {
		return ToolResult{IsError: true, Error: fmt.Sprintf("marshal args: %v", err)}, nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return ToolResult{IsError: true, Error: fmt.Sprintf("create request: %v", err)}, nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return ToolResult{IsError: true, Error: fmt.Sprintf("http request: %v", err)}, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ToolResult{IsError: true, Error: fmt.Sprintf("read response: %v", err)}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return ToolResult{IsError: true, Error: fmt.Sprintf("http %d: %s", resp.StatusCode, string(respBody))}, nil
	}

	var result ToolResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		// Try to parse as simple output map
		var output map[string]any
		if err2 := json.Unmarshal(respBody, &output); err2 == nil {
			result.Output = output
		} else {
			return ToolResult{IsError: true, Error: fmt.Sprintf("parse response: %v", err)}, nil
		}
	}

	return result, nil
}

// Schemas returns all tool schemas for LLM function calling.
func (r *ToolRegistry) Schemas() []ToolSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemas := make([]ToolSchema, 0, len(r.tools))
	for _, t := range r.tools {
		if t.Mode == ModeLib && t.ToolPlugin != nil {
			schemas = append(schemas, t.ToolPlugin.Schema())
		} else if t.Manifest.Tool != nil {
			schemas = append(schemas, t.Manifest.Tool.Schema)
		}
	}
	return schemas
}

// List returns all registered tool names.
func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ProviderRegistry manages loaded provider plugins.
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]ProviderPlugin
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]ProviderPlugin),
	}
}

// Register adds a loaded plugin to the provider registry.
func (r *ProviderRegistry) Register(loaded *LoadedPlugin) error {
	if loaded.Manifest.Type != TypeProvider {
		return fmt.Errorf("plugin %s is not a provider plugin", loaded.Manifest.Name)
	}

	if loaded.Mode != ModeLib {
		return fmt.Errorf("providers must use lib mode (plugin %s uses %s)", loaded.Manifest.Name, loaded.Mode)
	}

	pp, ok := loaded.Plugin.(ProviderPlugin)
	if !ok {
		return fmt.Errorf("plugin %s does not implement ProviderPlugin", loaded.Manifest.Name)
	}

	r.mu.Lock()
	r.providers[pp.ProviderID()] = pp
	r.mu.Unlock()

	return nil
}

// Unregister removes a provider from the registry.
func (r *ProviderRegistry) Unregister(providerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, providerID)
}

// Get returns a provider by ID.
func (r *ProviderRegistry) Get(providerID string) (ProviderPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[providerID]
	return p, ok
}

// List returns all registered provider IDs.
func (r *ProviderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

// ConfigureAll configures all providers with their API keys from environment variables.
func (r *ProviderRegistry) ConfigureAll(getEnv func(string) string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Note: We need access to the manifest to get auth_env
	// This should be called after loading, passing the env getter
	return nil
}
