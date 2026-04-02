// Package channel provides a multi-platform messaging interface for agent
// communication. Each platform (Web UI, CLI, Telegram, etc.) implements
// the Channel interface with its own capabilities and message handling.
package channel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/quarkloop/agent/pkg/agentcore"
	"github.com/quarkloop/agent/pkg/eventbus"
)

// ChannelCaps declares what a channel can do.
type ChannelCaps struct {
	CanStream     bool // can deliver partial output in real-time
	CanEdit       bool // can update previously sent messages
	CanDelete     bool // can remove messages
	CanReact      bool // can add emoji reactions
	CanShowTyping bool // can display typing indicator
	MaxLength     int  // max message length (0 = unlimited)
}

// Channel is the interface each platform implements.
type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Send(ctx context.Context, msg OutboundMessage) ([]string, error)
	IsRunning() bool
	IsAllowed(senderID string) bool
	Caps() ChannelCaps
}

// ActivityIndicator shows/hides typing indicators.
type ActivityIndicator interface {
	ShowTyping(ctx context.Context, chatID string) error
	HideTyping(ctx context.Context, chatID string) error
}

// MessageEditor can update previously sent messages.
type MessageEditor interface {
	EditMessage(ctx context.Context, messageID string, newContent string) error
}

// MessageRemover can delete messages.
type MessageRemover interface {
	DeleteMessage(ctx context.Context, messageID string) error
}

// ReactionEmitter can add emoji reactions.
type ReactionEmitter interface {
	AddReaction(ctx context.Context, messageID string, emoji string) error
}

// ProgressiveSender can stream output in chunks.
type ProgressiveSender interface {
	SendChunked(ctx context.Context, chatID string, chunks <-chan string) (string, error)
}

// InboundMessage is a message received from a platform.
type InboundMessage struct {
	ID        string
	ChannelID string
	ChatID    string
	SenderID  string
	Content   string
	Timestamp time.Time
	Metadata  map[string]string
}

// OutboundMessage is a message to send to a platform.
type OutboundMessage struct {
	ChatID  string
	Content string
	ReplyTo string // message ID to reply to
}

// Manager routes inbound messages through channels to agents.
type Manager struct {
	mu       sync.RWMutex
	channels map[string]Channel
	bus      *eventbus.Bus
}

// NewManager creates a channel manager.
func NewManager(bus *eventbus.Bus) *Manager {
	return &Manager{
		channels: make(map[string]Channel),
		bus:      bus,
	}
}

// Register adds a channel to the manager.
func (m *Manager) Register(ch Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.channels[ch.Name()]; exists {
		return fmt.Errorf("channel %q already registered", ch.Name())
	}
	m.channels[ch.Name()] = ch
	return nil
}

// Get returns a channel by name.
func (m *Manager) Get(name string) (Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ch, ok := m.channels[name]
	return ch, ok
}

// List returns all registered channel names.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	return names
}

// Route handles an inbound message: checks allowlist, sends response via channel.
func (m *Manager) Route(ctx context.Context, msg InboundMessage, chatFn func(ctx context.Context, content string) (*agentcore.ChatResponse, error)) error {
	ch, ok := m.Get(msg.ChannelID)
	if !ok {
		return fmt.Errorf("channel %q not found", msg.ChannelID)
	}

	if !ch.IsAllowed(msg.SenderID) {
		return fmt.Errorf("sender %q not allowed on channel %q", msg.SenderID, msg.ChannelID)
	}

	caps := ch.Caps()

	// Show typing indicator if supported.
	if caps.CanShowTyping {
		if ai, ok := ch.(ActivityIndicator); ok {
			ai.ShowTyping(ctx, msg.ChatID)
			defer ai.HideTyping(ctx, msg.ChatID)
		}
	}

	// Show reaction if supported.
	if caps.CanReact {
		if re, ok := ch.(ReactionEmitter); ok {
			re.AddReaction(ctx, msg.ID, "👀")
		}
	}

	// Process the message through the agent.
	resp, err := chatFn(ctx, msg.Content)
	if err != nil {
		return fmt.Errorf("chat: %w", err)
	}

	// Split message if channel has a max length.
	parts := splitMessage(resp.Reply, caps.MaxLength)
	var messageIDs []string
	for _, part := range parts {
		ids, err := ch.Send(ctx, OutboundMessage{
			ChatID:  msg.ChatID,
			Content: part,
			ReplyTo: msg.ID,
		})
		if err != nil {
			return fmt.Errorf("send: %w", err)
		}
		messageIDs = append(messageIDs, ids...)
	}

	m.bus.Emit(eventbus.Event{
		Kind:      eventbus.KindMessageAdded,
		SessionID: msg.ChatID,
		Data:      map[string]string{"channel": ch.Name(), "messages": fmt.Sprintf("%d", len(messageIDs))},
	})

	return nil
}

// Broadcast sends a message to all channels for a given chat context.
func (m *Manager) Broadcast(ctx context.Context, msg OutboundMessage) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.channels {
		if _, err := ch.Send(ctx, msg); err != nil {
			return fmt.Errorf("broadcast to %s: %w", ch.Name(), err)
		}
	}
	return nil
}

// StartAll starts all registered channels.
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.channels {
		if err := ch.Start(ctx); err != nil {
			return fmt.Errorf("start channel %s: %w", ch.Name(), err)
		}
	}
	return nil
}

// StopAll stops all registered channels.
func (m *Manager) StopAll(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.channels {
		ch.Stop(ctx)
	}
}

// splitMessage splits content into chunks of maxLen characters.
// If maxLen is 0, returns the content as a single chunk.
func splitMessage(content string, maxLen int) []string {
	if maxLen <= 0 || len(content) <= maxLen {
		return []string{content}
	}

	var parts []string
	for len(content) > 0 {
		end := maxLen
		if end > len(content) {
			end = len(content)
		}
		// Try to split at the last space before maxLen.
		if end < len(content) {
			if lastSpace := strings.LastIndex(content[:end], " "); lastSpace > 0 {
				end = lastSpace
			}
		}
		parts = append(parts, content[:end])
		content = strings.TrimLeft(content[end:], " ")
	}
	return parts
}
