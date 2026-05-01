package message

// Poster posts messages to the agent inbox.
type Poster interface {
	Post(sessionID, content string, resp chan StreamMessage)
}

// SessionAccess provides session state for message handlers.
type SessionAccess interface {
	Has(id string) bool
	GetMessages(id string) []Message
	Subscribe(id string) chan Message
	Unsubscribe(id string, ch chan Message)
}
