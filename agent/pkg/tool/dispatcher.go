package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Registry is a thread-safe tool registry with TTL expiry, hidden tools,
// tool groups, and usage statistics. It satisfies the Invoker interface.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*registeredTool
	http  *http.Client
}

type registeredTool struct {
	def   *Definition
	stats ToolStats
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*registeredTool),
		http:  &http.Client{Timeout: 120 * time.Second},
	}
}

// Register adds a tool. If TTL > 0, sets ExpiresAt.
func (r *Registry) Register(name string, def *Definition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	def.RegisteredAt = time.Now().UTC()
	if def.TTL > 0 {
		expiresAt := time.Now().Add(def.TTL)
		def.ExpiresAt = &expiresAt
	}
	r.tools[name] = &registeredTool{def: def}
}

// Deregister removes a tool by name.
func (r *Registry) Deregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Invoke calls the tool, tracks stats, and returns the result.
func (r *Registry) Invoke(ctx context.Context, name string, input map[string]any) (map[string]any, error) {
	r.mu.Lock()
	rt, ok := r.tools[name]
	if !ok {
		r.mu.Unlock()
		return nil, fmt.Errorf("tool %q not found", name)
	}

	if rt.def.ExpiresAt != nil && time.Now().After(*rt.def.ExpiresAt) {
		delete(r.tools, name)
		r.mu.Unlock()
		return nil, fmt.Errorf("tool %q has expired", name)
	}

	def := rt.def
	rt.stats.CallCount++
	rt.stats.LastCalled = time.Now()
	r.mu.Unlock()

	start := time.Now()
	out, err := r.httpInvoke(ctx, def, input)
	elapsed := time.Since(start)

	r.mu.Lock()
	rt.stats.TotalLatency += elapsed
	rt.stats.AvgLatencyMs = float64(rt.stats.TotalLatency.Milliseconds()) / float64(rt.stats.CallCount)
	if err != nil {
		rt.stats.ErrorCount++
	}
	r.mu.Unlock()

	return out, err
}

// List returns names of all non-expired, non-hidden tools.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.pruneExpiredLocked()
	names := make([]string, 0, len(r.tools))
	for name, rt := range r.tools {
		if rt.def.Hidden {
			continue
		}
		names = append(names, name)
	}
	return names
}

// ListAll returns all tool names including hidden ones.
func (r *Registry) ListAll() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.pruneExpiredLocked()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ListGroup returns tool names in the given group.
func (r *Registry) ListGroup(group string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name, rt := range r.tools {
		if rt.def.Group == group {
			names = append(names, name)
		}
	}
	return names
}

// EnableGroup makes all tools in the group callable (removes hidden flag).
func (r *Registry) EnableGroup(group string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rt := range r.tools {
		if rt.def.Group == group {
			rt.def.Hidden = false
		}
	}
}

// DisableGroup makes all tools in the group hidden.
func (r *Registry) DisableGroup(group string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rt := range r.tools {
		if rt.def.Group == group {
			rt.def.Hidden = true
		}
	}
}

// Stats returns usage stats for a tool.
func (r *Registry) Stats(name string) (*ToolStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rt, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return &rt.stats, nil
}

// PruneExpired removes all tools past their TTL.
func (r *Registry) PruneExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneExpiredLocked()
}

func (r *Registry) pruneExpiredLocked() {
	now := time.Now()
	for name, rt := range r.tools {
		if rt.def.ExpiresAt != nil && now.After(*rt.def.ExpiresAt) {
			delete(r.tools, name)
		}
	}
}

// Schema returns the tool's definition for LLM function calling.
func (r *Registry) Schema(name string) (*Definition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rt, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return rt.def, nil
}

// Schemas returns definitions for all visible (non-hidden, non-expired) tools.
func (r *Registry) Schemas() []*Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.pruneExpiredLocked()
	defs := make([]*Definition, 0, len(r.tools))
	for _, rt := range r.tools {
		if rt.def.Hidden {
			continue
		}
		defs = append(defs, rt.def)
	}
	return defs
}

func (r *Registry) httpInvoke(ctx context.Context, def *Definition, input map[string]any) (map[string]any, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal tool input: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, def.Endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create tool request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tool %q http error: %w", def.Name, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tool %q response: %w", def.Name, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tool %q returned %d: %s", def.Name, resp.StatusCode, string(data))
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding tool %q response: %w", def.Name, err)
	}
	return result, nil
}
