package agent

import (
	"sync"
)

// Registry tracks multiple agent instances by ID.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*Agent
}

// NewRegistry creates an empty agent registry.
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*Agent),
	}
}

// Get returns an agent by ID.
func (r *Registry) Get(agentID string) (*Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[agentID]
	return a, ok
}

// Register adds an agent to the registry.
func (r *Registry) Register(agentID string, a *Agent) {
	r.mu.Lock()
	r.agents[agentID] = a
	r.mu.Unlock()
}

// List returns all registered agent IDs.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.agents))
	for id := range r.agents {
		ids = append(ids, id)
	}
	return ids
}
