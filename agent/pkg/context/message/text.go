package message

// TextPayload is a plain-text conversational turn.
// Used for user messages, agent replies, and any prose exchange.
type TextPayload struct {
	// Text is the message body.
	Text string `json:"text"`
}

func init() { RegisterPayloadFactory(TextType, func() Payload { return &TextPayload{} }) }

func (p TextPayload) Kind() MessageType          { return TextType }
func (p TextPayload) TextRepresentation() string { return p.Text }
func (p TextPayload) sealPayload()               {}

// LLMText returns the full conversational text.
func (p TextPayload) LLMText() string { return p.Text }

// UserText returns the full text — users see the complete conversation.
func (p TextPayload) UserText() string { return p.Text }

// DevText returns the full text — same as UserText for plain turns.
func (p TextPayload) DevText() string { return p.Text }
