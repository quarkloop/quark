// Package resolver provides multi-agent message routing.
//
// Given an inbound message, the resolver determines which agent should
// handle it. Strategies include session affinity, channel-based routing,
// content classification, and fallback.
package resolver

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNoMatch is returned when a resolver cannot determine a target agent.
var ErrNoMatch = errors.New("no agent matched the routing criteria")

// InboundMessage carries the routing context for an incoming message.
type InboundMessage struct {
	Content    string            // message text
	SenderID   string            // platform-specific sender ID
	Channel    string            // "web", "telegram", "cli", etc.
	SessionKey string            // existing session key, if any
	Metadata   map[string]string // additional routing hints
}

// Resolver determines which agent should handle an inbound message.
type Resolver interface {
	Resolve(ctx context.Context, msg InboundMessage) (agentID string, err error)
}

// ResolverFunc adapts a plain function to the Resolver interface.
type ResolverFunc func(ctx context.Context, msg InboundMessage) (string, error)

func (f ResolverFunc) Resolve(ctx context.Context, msg InboundMessage) (string, error) {
	return f(ctx, msg)
}

// SessionAffinityResolver routes to the agent that owns the message's session.
// If the message has no session key, it returns ErrNoMatch.
type SessionAffinityResolver struct{}

// Resolve extracts the agent ID from the session key.
// Session keys follow the pattern: agent:<agentID>:<type>[:<id>]
func (r *SessionAffinityResolver) Resolve(ctx context.Context, msg InboundMessage) (string, error) {
	if msg.SessionKey == "" {
		return "", ErrNoMatch
	}
	// Parse agent:<agentID>:<type>[:<id>]
	parts := strings.SplitN(msg.SessionKey, ":", 4)
	if len(parts) < 3 || parts[0] != "agent" {
		return "", ErrNoMatch
	}
	return parts[1], nil
}

// ChannelResolver routes messages based on the channel they arrived from.
type ChannelResolver struct {
	routes map[string]string // channel name → agent ID
}

// NewChannelResolver creates a ChannelResolver with the given channel→agent mapping.
func NewChannelResolver(routes map[string]string) *ChannelResolver {
	return &ChannelResolver{routes: routes}
}

// Resolve returns the agent ID configured for the message's channel.
func (r *ChannelResolver) Resolve(ctx context.Context, msg InboundMessage) (string, error) {
	if msg.Channel == "" {
		return "", ErrNoMatch
	}
	agentID, ok := r.routes[msg.Channel]
	if !ok {
		return "", ErrNoMatch
	}
	return agentID, nil
}

// FallbackResolver always returns the configured default agent ID.
type FallbackResolver struct {
	DefaultAgentID string
}

// NewFallbackResolver creates a FallbackResolver.
func NewFallbackResolver(agentID string) *FallbackResolver {
	return &FallbackResolver{DefaultAgentID: agentID}
}

// Resolve always returns the default agent ID.
func (r *FallbackResolver) Resolve(ctx context.Context, msg InboundMessage) (string, error) {
	if r.DefaultAgentID == "" {
		return "", fmt.Errorf("no fallback agent configured")
	}
	return r.DefaultAgentID, nil
}

// ChainResolver tries resolvers in order and returns the first match.
type ChainResolver struct {
	resolvers []Resolver
}

// NewChainResolver creates a ChainResolver.
func NewChainResolver(resolvers ...Resolver) *ChainResolver {
	return &ChainResolver{resolvers: resolvers}
}

// Resolve tries each resolver in order. Returns the first successful match.
func (c *ChainResolver) Resolve(ctx context.Context, msg InboundMessage) (string, error) {
	for _, r := range c.resolvers {
		agentID, err := r.Resolve(ctx, msg)
		if err == nil && agentID != "" {
			return agentID, nil
		}
	}
	return "", fmt.Errorf("no resolver matched")
}
