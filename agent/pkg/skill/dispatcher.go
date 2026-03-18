package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Invoker defines the interface for invoking skills.
type Invoker interface {
	Register(name string, def *Definition)
	Invoke(ctx context.Context, name string, input map[string]interface{}) (map[string]interface{}, error)
	List() []string
}

// HTTPDispatcher invokes skills via their registered HTTP endpoints.
// Safe for concurrent use from multiple goroutines.
type HTTPDispatcher struct {
	mu     sync.RWMutex
	skills map[string]*Definition
	http   *http.Client
}

// NewHTTPDispatcher creates an empty HTTPDispatcher.
func NewHTTPDispatcher() *HTTPDispatcher {
	return &HTTPDispatcher{
		skills: map[string]*Definition{},
		http:   &http.Client{},
	}
}

// Register adds a skill to the dispatcher. Safe for concurrent use.
func (d *HTTPDispatcher) Register(name string, def *Definition) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.skills[name] = def
}

// Invoke calls the named skill's HTTP endpoint with the given input.
// Returns the parsed JSON response or an error.
func (d *HTTPDispatcher) Invoke(ctx context.Context, name string, input map[string]interface{}) (map[string]interface{}, error) {
	d.mu.RLock()
	def, ok := d.skills[name]
	d.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("skill %q not registered", name)
	}
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, def.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoking skill %q: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skill %q returned HTTP %d", name, resp.StatusCode)
	}
	var out map[string]interface{}
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// List returns the names of all registered skills.
func (d *HTTPDispatcher) List() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	names := make([]string, 0, len(d.skills))
	for n := range d.skills {
		names = append(names, n)
	}
	return names
}
