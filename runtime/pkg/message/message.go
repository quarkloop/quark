// Package message provides message types and the message handling flow.
package message

import "github.com/quarkloop/pkg/plugin"

// Message is an alias for plugin.Message, the canonical message type.
type Message = plugin.Message

// StreamMessage defines a typed streaming message sent back to the channel.
type StreamMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// UserMessage is an incoming message from any channel.
type UserMessage struct {
	SessionID string
	Content   string
	Response  chan StreamMessage // tokens streamed back; closed when done
}
