// Package registry tracks running agent processes managed by the supervisor.
// The registry is purely in-memory: it does not persist state. Durable data
// (spaces, Quarkfiles, KB) lives in the space.Store.
package registry

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/quarkloop/supervisor/pkg/api"
)

// Agent is the runtime record for a running agent process.
type Agent struct {
	ID         string
	Space      string
	WorkingDir string
	PluginsDir string
	Status     api.AgentStatus
	PID        int
	Port       int
	StartedAt  time.Time
	Cmd        *exec.Cmd
}

// Registry tracks running agent processes keyed by agent ID.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*Agent
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{agents: make(map[string]*Agent)}
}

// Register adds an agent entry.
func (r *Registry) Register(a *Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[a.ID] = a
}

// Get returns the agent with the given ID or an error if it does not exist.
func (r *Registry) Get(id string) (*Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[id]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", id)
	}
	return a, nil
}

// GetBySpace returns the first agent running for the given space name, or an
// error if no agent is running for it.
func (r *Registry) GetBySpace(space string) (*Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, a := range r.agents {
		if a.Space == space {
			return a, nil
		}
	}
	return nil, fmt.Errorf("no agent running for space %q", space)
}

// List returns a snapshot of all tracked agents.
func (r *Registry) List() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Agent, 0, len(r.agents))
	for _, a := range r.agents {
		out = append(out, a)
	}
	return out
}

// Remove drops an agent from the registry. It does not stop the process.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.agents, id)
}

// SetStatus updates the status of an agent.
func (r *Registry) SetStatus(id string, status api.AgentStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.agents[id]
	if !ok {
		return fmt.Errorf("agent %q not found", id)
	}
	a.Status = status
	return nil
}
