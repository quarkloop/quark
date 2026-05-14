// Package message provides message types and the message handling flow.
package message

import "context"

// Message is the runtime session message shape. Provider-specific chat wire
// messages are mapped at the inference boundary.
type Message struct {
	ID        string `json:"id,omitempty"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

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

// Emit sends a stream message unless the request context has been cancelled.
func Emit(ctx context.Context, ch chan<- StreamMessage, msg StreamMessage) bool {
	select {
	case ch <- msg:
		return true
	case <-ctx.Done():
		return false
	}
}
