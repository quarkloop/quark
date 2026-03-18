package message

import "fmt"

// ReasoningPayload holds the agent's internal chain-of-thought or scratchpad.
//
// Visible only to developers by default.  May be included in the LLM context
// when using models that support explicit reasoning steps (e.g. o1, extended
// thinking), controlled by the IsInternal flag.
type ReasoningPayload struct {
	// Reasoning is the raw scratchpad or chain-of-thought text.
	Reasoning string `json:"reasoning"`
	// StepNumber is the zero-based index of this step in a multi-step plan.
	StepNumber int `json:"step_number,omitempty"`
	// IsInternal marks reasoning that must never be forwarded to the LLM.
	IsInternal bool `json:"is_internal,omitempty"`
}

func init() { RegisterPayloadFactory(ReasoningType, func() Payload { return &ReasoningPayload{} }) }

func (p ReasoningPayload) Kind() MessageType { return ReasoningType }
func (p ReasoningPayload) sealPayload()      {}

func (p ReasoningPayload) TextRepresentation() string {
	return fmt.Sprintf("[reasoning step=%d] %s", p.StepNumber, p.Reasoning)
}

// LLMText returns "" for internal reasoning; the scratchpad text otherwise.
func (p ReasoningPayload) LLMText() string {
	if p.IsInternal {
		return ""
	}
	return p.Reasoning
}

// UserText returns "" — chain-of-thought is not for users.
func (p ReasoningPayload) UserText() string { return "" }

// DevText returns step context and the full reasoning text.
func (p ReasoningPayload) DevText() string {
	internal := ""
	if p.IsInternal {
		internal = " [INTERNAL]"
	}
	return fmt.Sprintf("[reasoning step=%d%s]\n%s", p.StepNumber, internal, p.Reasoning)
}
