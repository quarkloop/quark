// Package message provides message types and the message handling flow.
package message

// Message represents a single chat message in a session.
type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
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
