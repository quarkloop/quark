package channel

import (
	"context"
	"sync"
)

// WebChannel is the default channel for HTTP/Web UI communication.
// It supports streaming and editing but has no platform-specific limits.
type WebChannel struct {
	mu        sync.RWMutex
	running   bool
	allowlist []string
}

// NewWebChannel creates a Web UI channel.
func NewWebChannel(allowlist []string) *WebChannel {
	return &WebChannel{allowlist: allowlist}
}

// Name returns the channel name.
func (w *WebChannel) Name() string { return "web" }

// Start marks the channel as running.
func (w *WebChannel) Start(ctx context.Context) error {
	w.mu.Lock()
	w.running = true
	w.mu.Unlock()
	return nil
}

// Stop marks the channel as stopped.
func (w *WebChannel) Stop(ctx context.Context) error {
	w.mu.Lock()
	w.running = false
	w.mu.Unlock()
	return nil
}

// Send is a no-op for the Web channel — the HTTP handler returns the
// response directly. This method exists to satisfy the Channel interface.
func (w *WebChannel) Send(ctx context.Context, msg OutboundMessage) ([]string, error) {
	return []string{}, nil
}

// IsRunning returns whether the channel is active.
func (w *WebChannel) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// IsAllowed checks the sender against the allowlist.
// Empty allowlist means all senders are allowed.
func (w *WebChannel) IsAllowed(senderID string) bool {
	if len(w.allowlist) == 0 {
		return true
	}
	for _, allowed := range w.allowlist {
		if allowed == senderID {
			return true
		}
	}
	return false
}

// Caps returns the Web channel capabilities.
func (w *WebChannel) Caps() ChannelCaps {
	return ChannelCaps{
		CanStream:     true,
		CanEdit:       true,
		CanDelete:     true,
		CanReact:      false,
		CanShowTyping: true,
		MaxLength:     0, // unlimited
	}
}
