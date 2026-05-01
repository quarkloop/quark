package channel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// ChannelBus registers, starts, stops, and manages channels.
type ChannelBus struct {
	mu       sync.RWMutex
	channels map[ChannelType]Channel
}

// NewChannelBus creates a new ChannelBus.
func NewChannelBus() *ChannelBus {
	return &ChannelBus{channels: make(map[ChannelType]Channel)}
}

// Register adds a channel to the bus.
func (b *ChannelBus) Register(ch Channel) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.channels[ch.Type()] = ch
	slog.Info("channel registered", "type", ch.Type())
}

// Start starts all registered channels.
func (b *ChannelBus) Start(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.channels {
		if err := ch.Start(ctx); err != nil {
			return fmt.Errorf("start %s: %w", ch.Type(), err)
		}
		slog.Info("channel started", "type", ch.Type())
	}
	return nil
}

// Stop stops all registered channels.
func (b *ChannelBus) Stop(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var lastErr error
	for _, ch := range b.channels {
		if err := ch.Stop(ctx); err != nil {
			slog.Error("channel stop error", "type", ch.Type(), "error", err)
			lastErr = err
		}
	}
	return lastErr
}

// Get returns a registered channel by type.
func (b *ChannelBus) Get(channelType ChannelType) Channel {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.channels[channelType]
}

// ActiveChannels returns info about currently registered channels.
func (b *ChannelBus) ActiveChannels() []ChannelInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]ChannelInfo, 0, len(b.channels))
	for ct := range b.channels {
		out = append(out, ChannelInfo{Type: ct, Active: true})
	}
	return out
}

// AvailableChannels returns all known channel types with their active status.
func (b *ChannelBus) AvailableChannels() []ChannelInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]ChannelInfo, len(AllChannelTypes))
	for i, ct := range AllChannelTypes {
		_, active := b.channels[ct]
		out[i] = ChannelInfo{Type: ct, Active: active}
	}
	return out
}
