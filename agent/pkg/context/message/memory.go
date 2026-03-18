package message

import "fmt"

// MemoryScope defines the lifetime and retrieval scope of a memory entry.
type MemoryScope string

const (
	// MemoryScopeEphemeral lives for a single turn and is not persisted.
	MemoryScopeEphemeral MemoryScope = "ephemeral"
	// MemoryScopeSession persists for the duration of a conversation session.
	MemoryScopeSession MemoryScope = "session"
	// MemoryScopeUser persists across sessions for a specific user.
	MemoryScopeUser MemoryScope = "user"
	// MemoryScopeGlobal is shared across all users of the agent.
	MemoryScopeGlobal MemoryScope = "global"
)

// MemoryPayload carries agent memory: retrieved facts, conversation summaries,
// or user preferences injected into the context by the memory subsystem.
//
// Memory messages are visible to the LLM but not to the end user by default.
type MemoryPayload struct {
	// Summary is the memory text injected into the LLM context.
	Summary string `json:"summary"`
	// Scope defines the lifetime and retrieval boundary.
	Scope MemoryScope `json:"scope"`
	// SourceMessageIDs optionally lists messages condensed into this entry.
	SourceMessageIDs []string `json:"source_message_ids,omitempty"`
	// Confidence is a [0,1] retrieval confidence score from the memory store.
	Confidence float64 `json:"confidence,omitempty"`
	// Tags are optional labels for filtering memory during retrieval.
	Tags []string `json:"tags,omitempty"`
}

func init() { RegisterPayloadFactory(MemoryType, func() Payload { return &MemoryPayload{} }) }

func (p MemoryPayload) Kind() MessageType          { return MemoryType }
func (p MemoryPayload) sealPayload()               {}
func (p MemoryPayload) TextRepresentation() string { return fmt.Sprintf("[memory:%s] %s", p.Scope, p.Summary) }

// LLMText injects the summary silently into the model context.
func (p MemoryPayload) LLMText() string { return p.Summary }

// UserText returns "" — raw memory entries are not shown to users.
func (p MemoryPayload) UserText() string { return "" }

// DevText returns full memory metadata for developer tooling.
func (p MemoryPayload) DevText() string {
	return fmt.Sprintf("[memory scope=%s confidence=%.2f tags=%v]\n%s",
		p.Scope, p.Confidence, p.Tags, p.Summary)
}
