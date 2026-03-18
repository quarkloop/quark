package llmctx

// =============================================================================
// builder.go
//
// AgentContextBuilder constructs a validated AgentContext via a fluent API.
//
// Why a builder?
//   - Makes required vs. optional fields self-documenting at the call site.
//   - All validation is centralised in Build(); no nil-checks scatter elsewhere.
//   - New options can be added as With* methods without breaking call sites.
//
// Usage:
//
//	ac, err := llmctx.NewAgentContextBuilder().
//	    WithSystemPrompt(sysMsg).
//	    WithContextWindow(window).
//	    WithCompactor(compactor).
//	    WithTokenComputer(tc).
//	    Build()
// =============================================================================

// AgentContextBuilder accumulates configuration for an AgentContext.
// The zero value is usable; call Build() after configuring at minimum a
// TokenComputer.
type AgentContextBuilder struct {
	systemPrompt *Message
	window       ContextWindow
	compactor    Compactor
	tc           TokenComputer
	idGen        IDGenerator
}

// NewAgentContextBuilder returns a fresh builder with no configuration set.
func NewAgentContextBuilder() *AgentContextBuilder { return &AgentContextBuilder{} }

// WithSystemPrompt sets the system prompt message.
// The system prompt is prepended to the message list and is protected from
// removal and compaction eviction.
func (b *AgentContextBuilder) WithSystemPrompt(m *Message) *AgentContextBuilder {
	b.systemPrompt = m
	return b
}

// WithContextWindow sets the token budget. A zero value means unbounded.
func (b *AgentContextBuilder) WithContextWindow(w ContextWindow) *AgentContextBuilder {
	b.window = w
	return b
}

// WithCompactor sets the compaction strategy.
// Optional: if not set, AgentContext.Compact() returns ErrCodeNoCompactor.
func (b *AgentContextBuilder) WithCompactor(c Compactor) *AgentContextBuilder {
	b.compactor = c
	return b
}

// WithTokenComputer sets the token counting implementation.
// Required: Build() returns ErrCodeInvalidConfig when nil.
func (b *AgentContextBuilder) WithTokenComputer(tc TokenComputer) *AgentContextBuilder {
	b.tc = tc
	return b
}

// WithIDGenerator sets the strategy used to generate MessageIDs for messages
// created by the AgentContext itself (e.g. in future factory helpers).
// Optional: defaults to UUIDIDGenerator when not set.
func (b *AgentContextBuilder) WithIDGenerator(g IDGenerator) *AgentContextBuilder {
	b.idGen = g
	return b
}

// Build validates the configuration and returns a ready AgentContext.
// Returns *ContextError with ErrCodeInvalidConfig if required fields are missing.
func (b *AgentContextBuilder) Build() (*AgentContext, error) {
	if b.tc == nil {
		return nil, newErr(ErrCodeInvalidConfig,
			"TokenComputer is required; call WithTokenComputer before Build", nil)
	}
	idGen := b.idGen
	if idGen == nil {
		idGen = DefaultIDGenerator()
	}
	ac := &AgentContext{
		systemPrompt:  b.systemPrompt,
		messages:      make([]*Message, 0),
		index:         make(map[string]int),
		contextWindow: b.window,
		compactor:     b.compactor,
		tc:            b.tc,
		idGen:         idGen,
		tput:          newThroughputTracker(),
	}
	if b.systemPrompt != nil {
		ac.messages = append(ac.messages, b.systemPrompt)
		ac.index[b.systemPrompt.id.value] = 0
		ac.cachedTokens = b.systemPrompt.tokenCount
		ac.tput.recordAppend()
	}
	return ac, nil
}
