package loop

// Message is the interface that all loop messages must implement.
// Type returns a string identifier used for handler dispatch.
// Priority returns an integer where higher values mean higher priority.
type Message interface {
	// Type returns the message type string used for handler dispatch.
	Type() string

	// Priority returns the message priority. Higher values are processed first.
	// Messages with priority > 0 are placed in the work queue.
	// Messages with priority <= 0 go to the regular inbox.
	Priority() int
}

// BaseMessage provides a simple Message implementation.
// Embed this in custom message types for convenience.
type BaseMessage struct {
	MsgType     string
	MsgPriority int
}

// Type returns the message type.
func (m BaseMessage) Type() string { return m.MsgType }

// Priority returns the message priority.
func (m BaseMessage) Priority() int { return m.MsgPriority }

// NewMessage creates a BaseMessage with the given type and zero priority.
func NewMessage(msgType string) BaseMessage {
	return BaseMessage{MsgType: msgType}
}

// NewPriorityMessage creates a BaseMessage with the given type and priority.
func NewPriorityMessage(msgType string, priority int) BaseMessage {
	return BaseMessage{MsgType: msgType, MsgPriority: priority}
}
