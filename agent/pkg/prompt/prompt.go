// Package prompt provides the embedded system prompt for the agent.
package prompt

import (
	"sync"

	_ "embed"
)

//go:embed systemprompt.md
var systemPrompt string

var mu sync.RWMutex

// SetSystemPrompt sets the system prompt (thread-safe).
func SetSystemPrompt(s string) {
	mu.Lock()
	defer mu.Unlock()
	systemPrompt = s
}

// GetSystemPrompt returns the system prompt (thread-safe).
func GetSystemPrompt() string {
	mu.RLock()
	defer mu.RUnlock()
	return systemPrompt
}
