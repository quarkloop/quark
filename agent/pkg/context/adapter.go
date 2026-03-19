package llmctx

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// LLM Adapter  (R4 thread-safe registry, R7 RequestOptions)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// LLMRole
// ---------------------------------------------------------------------------

// LLMRole is the canonical multi-turn conversation role vocabulary.
type LLMRole string

const (
	LLMRoleSystem    LLMRole = "system"
	LLMRoleUser      LLMRole = "user"
	LLMRoleAssistant LLMRole = "assistant"
	LLMRoleTool      LLMRole = "tool"
)

// ---------------------------------------------------------------------------
// LLMMessage
// ---------------------------------------------------------------------------

// LLMMessage is the normalised representation exchanged between AgentContext
// and an LLMAdapter. Adapters map from this type to their own wire format.
type LLMMessage struct {
	Role       LLMRole `json:"role"`
	Content    string  `json:"content"`
	ToolCallID string  `json:"tool_call_id,omitempty"`
	Name       string  `json:"name,omitempty"`
}

// ---------------------------------------------------------------------------
// RequestOptions  (R7)
// ---------------------------------------------------------------------------

// RequestOptions carries all parameters for a single LLM API call.
// Replace the previous four-positional-argument BuildRequest signature.
// Zero values are sane defaults; callers set only what they need.
// New fields can be added without breaking existing call sites.
type RequestOptions struct {
	// Model is the provider-specific model identifier (e.g. "gpt-4o", "claude-opus-4-6").
	// Required.
	Model string

	// MaxTokens caps the number of generated tokens (0 = provider default).
	MaxTokens int

	// Temperature controls sampling randomness (nil = provider default).
	Temperature *float32

	// TopP sets nucleus sampling probability (nil = provider default).
	TopP *float32

	// Stop is a list of sequences that halt generation (nil = no stop words).
	Stop []string

	// Stream requests token-by-token streaming when true.
	Stream bool

	// Extra carries any provider-specific fields not modelled above.
	// Values are merged into the top-level JSON object at serialisation time.
	Extra map[string]any
}

// ---------------------------------------------------------------------------
// LLMAdapter interface
// ---------------------------------------------------------------------------

// LLMAdapter converts an ordered list of Messages into the wire payload
// expected by a specific LLM provider.
//
// Adding a new provider requires only implementing this interface; no existing
// code needs to change.
type LLMAdapter interface {
	// Provider returns a stable identifier (e.g. "openai", "anthropic").
	Provider() string

	// BuildMessages converts domain Messages into normalised LLMMessages.
	BuildMessages(messages []*Message) ([]LLMMessage, error)

	// BuildRequest produces the serialised JSON body ready to POST to the API.
	BuildRequest(messages []LLMMessage, opts RequestOptions) ([]byte, error)

	// RoleFor maps a domain (MessageAuthor, MessageType) pair to an LLMRole.
	// Exposed so callers can inspect the mapping without deserialising bytes.
	RoleFor(author MessageAuthor, msgType MessageType) LLMRole
}

// ---------------------------------------------------------------------------
// defaultRoleFor – shared fallback mapping
// ---------------------------------------------------------------------------

func defaultRoleFor(author MessageAuthor, msgType MessageType) LLMRole {
	if msgType == SystemPromptType {
		return LLMRoleSystem
	}
	switch author {
	case AgentAuthor:
		return LLMRoleAssistant
	case UserAuthor:
		return LLMRoleUser
	case ToolAuthor:
		return LLMRoleTool
	case SystemAuthor:
		return LLMRoleSystem
	default:
		return LLMRoleUser
	}
}

// ---------------------------------------------------------------------------
// OpenAIAdapter
// ---------------------------------------------------------------------------

type openAIMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

// openAIPayload is the full chat-completions request body.
// extra fields are flattened into the JSON object at marshal time.
type openAIPayload struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float32        `json:"temperature,omitempty"`
	TopP        *float32        `json:"top_p,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	extra       map[string]any
}

func (p openAIPayload) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"model":    p.Model,
		"messages": p.Messages,
	}
	if p.MaxTokens > 0 {
		m["max_tokens"] = p.MaxTokens
	}
	if p.Temperature != nil {
		m["temperature"] = *p.Temperature
	}
	if p.TopP != nil {
		m["top_p"] = *p.TopP
	}
	if len(p.Stop) > 0 {
		m["stop"] = p.Stop
	}
	if p.Stream {
		m["stream"] = true
	}
	for k, v := range p.extra {
		m[k] = v
	}
	return json.Marshal(m)
}

// OpenAIAdapter serialises for the OpenAI chat-completions API.
type OpenAIAdapter struct{}

func (OpenAIAdapter) Provider() string { return "openai" }

// ZhipuAdapter uses the identical OpenAI wire format but identifies as "zhipu".
type ZhipuAdapter struct{ OpenAIAdapter }

func (ZhipuAdapter) Provider() string { return "zhipu" }

// OpenRouterAdapter uses the identical OpenAI wire format but identifies as "openrouter".
type OpenRouterAdapter struct{ OpenAIAdapter }

func (OpenRouterAdapter) Provider() string { return "openrouter" }

func (OpenAIAdapter) RoleFor(author MessageAuthor, msgType MessageType) LLMRole {
	return defaultRoleFor(author, msgType)
}

func (a OpenAIAdapter) BuildMessages(messages []*Message) ([]LLMMessage, error) {
	out := make([]LLMMessage, 0, len(messages))
	for _, m := range messages {
		if !m.IsVisibleTo(VisibleToLLM) {
			continue // respect visibility policy
		}
		out = append(out, LLMMessage{
			Role:    a.RoleFor(m.Author(), m.Type()),
			Content: m.LLMContent(), // use LLM-specific content representation
		})
	}
	return out, nil
}

func (a OpenAIAdapter) BuildRequest(messages []LLMMessage, opts RequestOptions) ([]byte, error) {
	if opts.Model == "" {
		return nil, newErr(ErrCodeSerializationFailed, "openai: Model must not be empty", nil)
	}
	wire := make([]openAIMessage, 0, len(messages))
	for _, msg := range messages {
		wire = append(wire, openAIMessage{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			Name:       msg.Name,
		})
	}
	payload := openAIPayload{
		Model:       opts.Model,
		Messages:    wire,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		TopP:        opts.TopP,
		Stop:        opts.Stop,
		Stream:      opts.Stream,
		extra:       opts.Extra,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, newErr(ErrCodeSerializationFailed,
			fmt.Sprintf("openai: failed to marshal request for model %q", opts.Model), err)
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// AnthropicAdapter
// ---------------------------------------------------------------------------

type anthropicContentBlock struct {
	Type string `json:"type"` // "text" | "image" | "tool_use" | "tool_result"
	Text string `json:"text,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"` // "user" | "assistant"
	Content []anthropicContentBlock `json:"content"`
}

type anthropicPayload struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	extra     map[string]any
}

func (p anthropicPayload) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"model":      p.Model,
		"messages":   p.Messages,
		"max_tokens": p.MaxTokens,
	}
	if p.System != "" {
		m["system"] = p.System
	}
	for k, v := range p.extra {
		m[k] = v
	}
	return json.Marshal(m)
}

// AnthropicAdapter serialises for the Anthropic Messages API.
//
// Provider-specific handling:
//   - System prompt → top-level "system" field (not in messages array).
//   - Only "user" and "assistant" roles exist.
//   - Consecutive same-role messages are merged with a newline.
//   - max_tokens defaults to 1024 when not provided (Anthropic requires it).
type AnthropicAdapter struct{}

func (AnthropicAdapter) Provider() string { return "anthropic" }

func (AnthropicAdapter) RoleFor(author MessageAuthor, msgType MessageType) LLMRole {
	if msgType == SystemPromptType {
		return LLMRoleSystem
	}
	if author == AgentAuthor {
		return LLMRoleAssistant
	}
	return LLMRoleUser // user, tool, unknown → "user"
}

func (a AnthropicAdapter) BuildMessages(messages []*Message) ([]LLMMessage, error) {
	out := make([]LLMMessage, 0, len(messages))
	for _, m := range messages {
		if !m.IsVisibleTo(VisibleToLLM) {
			continue
		}
		role := a.RoleFor(m.Author(), m.Type())
		if role == LLMRoleSystem {
			continue // system prompt travels in top-level "system" field
		}
		out = append(out, LLMMessage{
			Role:    role,
			Content: m.LLMContent(),
		})
	}
	return out, nil
}

func (a AnthropicAdapter) BuildRequest(messages []LLMMessage, opts RequestOptions) ([]byte, error) {
	if opts.Model == "" {
		return nil, newErr(ErrCodeSerializationFailed, "anthropic: Model must not be empty", nil)
	}
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}

	// Merge consecutive same-role messages into multi-block content arrays.
	var merged []anthropicMessage
	for _, msg := range messages {
		if msg.Role == LLMRoleSystem {
			continue
		}
		role := string(msg.Role)
		block := anthropicContentBlock{Type: "text", Text: msg.Content}
		if len(merged) > 0 && merged[len(merged)-1].Role == role {
			merged[len(merged)-1].Content = append(merged[len(merged)-1].Content, block)
		} else {
			merged = append(merged, anthropicMessage{
				Role:    role,
				Content: []anthropicContentBlock{block},
			})
		}
	}

	// System prompt may arrive via Extra["system"] from ContextAdapter.
	systemPrompt, _ := opts.Extra["system"].(string)

	extra := make(map[string]any, len(opts.Extra))
	for k, v := range opts.Extra {
		extra[k] = v
	}
	delete(extra, "system") // avoid duplication in marshalled output

	payload := anthropicPayload{
		Model:     opts.Model,
		System:    systemPrompt,
		Messages:  merged,
		MaxTokens: maxTokens,
		extra:     extra,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, newErr(ErrCodeSerializationFailed,
			fmt.Sprintf("anthropic: failed to marshal request for model %q", opts.Model), err)
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// GenericAdapter
// ---------------------------------------------------------------------------

// RoleMapFunc maps a domain role pair to a provider role string.
type RoleMapFunc func(author MessageAuthor, msgType MessageType) string

type genericWireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GenericAdapter targets any provider not covered by the built-in adapters.
// Supply a RoleMapper and optionally a PayloadBuilder for full control.
type GenericAdapter struct {
	ProviderName   string
	RoleMapper     RoleMapFunc
	PayloadBuilder func(messages []genericWireMessage, opts RequestOptions) ([]byte, error)
}

func (g GenericAdapter) Provider() string { return g.ProviderName }

func (g GenericAdapter) RoleFor(author MessageAuthor, msgType MessageType) LLMRole {
	if g.RoleMapper != nil {
		return LLMRole(g.RoleMapper(author, msgType))
	}
	return defaultRoleFor(author, msgType)
}

func (g GenericAdapter) BuildMessages(messages []*Message) ([]LLMMessage, error) {
	out := make([]LLMMessage, 0, len(messages))
	for _, m := range messages {
		if !m.IsVisibleTo(VisibleToLLM) {
			continue
		}
		out = append(out, LLMMessage{
			Role:    g.RoleFor(m.Author(), m.Type()),
			Content: m.LLMContent(),
		})
	}
	return out, nil
}

func (g GenericAdapter) BuildRequest(messages []LLMMessage, opts RequestOptions) ([]byte, error) {
	if opts.Model == "" {
		return nil, newErr(ErrCodeSerializationFailed,
			fmt.Sprintf("%s: Model must not be empty", g.ProviderName), nil)
	}
	wire := make([]genericWireMessage, 0, len(messages))
	for _, msg := range messages {
		wire = append(wire, genericWireMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	if g.PayloadBuilder != nil {
		data, err := g.PayloadBuilder(wire, opts)
		if err != nil {
			return nil, newErr(ErrCodeSerializationFailed,
				fmt.Sprintf("%s: payload builder failed", g.ProviderName), err)
		}
		return data, nil
	}

	// Default envelope: model + messages + known opts fields + Extra flattened.
	m := map[string]any{
		"model":    opts.Model,
		"messages": wire,
	}
	if opts.MaxTokens > 0 {
		m["max_tokens"] = opts.MaxTokens
	}
	if opts.Temperature != nil {
		m["temperature"] = *opts.Temperature
	}
	if opts.TopP != nil {
		m["top_p"] = *opts.TopP
	}
	if len(opts.Stop) > 0 {
		m["stop"] = opts.Stop
	}
	if opts.Stream {
		m["stream"] = true
	}
	for k, v := range opts.Extra {
		m[k] = v
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, newErr(ErrCodeSerializationFailed,
			fmt.Sprintf("%s: failed to marshal request", g.ProviderName), err)
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// AdapterRegistry  (R4: concurrency-safe)
// ---------------------------------------------------------------------------

// AdapterRegistry is a concurrency-safe registry of named LLMAdapters.
// R4: protected by sync.RWMutex — Register and Get may be called concurrently.
type AdapterRegistry struct {
	mu       sync.RWMutex // R4
	adapters map[string]LLMAdapter
}

// NewAdapterRegistry returns a registry pre-populated with default providers.
func NewAdapterRegistry() *AdapterRegistry {
	r := &AdapterRegistry{adapters: make(map[string]LLMAdapter)}
	r.Register(OpenAIAdapter{})
	r.Register(AnthropicAdapter{})
	r.Register(ZhipuAdapter{})
	r.Register(OpenRouterAdapter{})
	r.Register(NoopAdapter{})
	return r
}

// Register adds or replaces an adapter. Safe for concurrent use.
func (r *AdapterRegistry) Register(a LLMAdapter) {
	r.mu.Lock() // R4
	defer r.mu.Unlock()
	r.adapters[a.Provider()] = a
}

// Get retrieves the adapter for provider. Returns ErrCodeSerializationFailed
// if provider is not registered. Safe for concurrent use.
func (r *AdapterRegistry) Get(provider string) (LLMAdapter, error) {
	r.mu.RLock() // R4
	defer r.mu.RUnlock()
	a, ok := r.adapters[provider]
	if !ok {
		return nil, newErr(ErrCodeSerializationFailed,
			fmt.Sprintf("no adapter registered for provider %q", provider), nil)
	}
	return a, nil
}

// Providers returns the list of registered provider names.
// Safe for concurrent use.
func (r *AdapterRegistry) Providers() []string {
	r.mu.RLock() // R4
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.adapters))
	for k := range r.adapters {
		out = append(out, k)
	}
	return out
}

// ---------------------------------------------------------------------------
// ContextAdapter
// ---------------------------------------------------------------------------

// ContextAdapter couples an AgentContext to a specific LLMAdapter, providing
// a single BuildRequest call that handles the full pipeline.
type ContextAdapter struct {
	ctx     *AgentContext
	adapter LLMAdapter
}

// NewContextAdapter creates a ContextAdapter.
func NewContextAdapter(ctx *AgentContext, adapter LLMAdapter) *ContextAdapter {
	return &ContextAdapter{ctx: ctx, adapter: adapter}
}

// BuildRequest produces the serialised API payload using RequestOptions. (R7)
//
// R14 fix: passes only LLM-visible messages (VisibleToLLM) to BuildMessages.
// Previously the full message slice was passed and each adapter was expected to
// filter; custom adapters that omitted the check would silently leak dev-only or
// user-only messages into the model context.  The correct contract is: filtering
// happens before the adapter layer, not inside it.
func (ca *ContextAdapter) BuildRequest(opts RequestOptions) ([]byte, error) {
	messages := ca.ctx.LLMMessages() // R14: LLMMessages() not Messages()
	llmMessages, err := ca.adapter.BuildMessages(messages)
	if err != nil {
		return nil, newErr(ErrCodeSerializationFailed, "failed to build LLM messages", err)
	}

	// Inject system prompt text for adapters that need it as Extra["system"]
	// (e.g. Anthropic) without coupling ContextAdapter to any specific provider.
	if opts.Extra == nil {
		opts.Extra = make(map[string]any)
	}
	if sp := ca.ctx.SystemPrompt(); sp != nil {
		if _, exists := opts.Extra["system"]; !exists {
			opts.Extra["system"] = sp.LLMContent()
		}
	}

	return ca.adapter.BuildRequest(llmMessages, opts)
}

// Provider returns the underlying adapter's provider name.
func (ca *ContextAdapter) Provider() string { return ca.adapter.Provider() }

// ---------------------------------------------------------------------------
// MergeLLMMessages helper
// ---------------------------------------------------------------------------

// MergeLLMMessages collapses consecutive messages with the same role,
// joining their content with separator (defaults to "\n").
func MergeLLMMessages(messages []LLMMessage, separator string) []LLMMessage {
	if len(messages) == 0 {
		return messages
	}
	if separator == "" {
		separator = "\n"
	}
	merged := []LLMMessage{messages[0]}
	for _, m := range messages[1:] {
		last := &merged[len(merged)-1]
		if last.Role == m.Role {
			last.Content = strings.Join([]string{last.Content, m.Content}, separator)
		} else {
			merged = append(merged, m)
		}
	}
	return merged
}

// ---------------------------------------------------------------------------
// NoopAdapter — for dry-run / testing without a real API
// ---------------------------------------------------------------------------

// NoopAdapter satisfies LLMAdapter without producing any real API payload.
// It serialises a minimal JSON body that the noopGateway in pkg/model
// can echo back, allowing the full agent pipeline to run without credentials.
type NoopAdapter struct{}

func (NoopAdapter) Provider() string { return "noop" }

func (NoopAdapter) RoleFor(author MessageAuthor, msgType MessageType) LLMRole {
	return defaultRoleFor(author, msgType)
}

func (NoopAdapter) BuildMessages(messages []*Message) ([]LLMMessage, error) {
	out := make([]LLMMessage, 0, len(messages))
	for _, m := range messages {
		out = append(out, LLMMessage{
			Role:    defaultRoleFor(m.Author(), m.Type()),
			Content: m.LLMContent(),
		})
	}
	return out, nil
}

func (NoopAdapter) BuildRequest(messages []LLMMessage, opts RequestOptions) ([]byte, error) {
	type noopMsg struct {
		Role    LLMRole `json:"role"`
		Content string  `json:"content"`
	}
	type noopPayload struct {
		Model     string    `json:"model"`
		Messages  []noopMsg `json:"messages"`
		MaxTokens int       `json:"max_tokens,omitempty"`
	}
	msgs := make([]noopMsg, len(messages))
	for i, m := range messages {
		msgs[i] = noopMsg{Role: m.Role, Content: m.Content}
	}
	p := noopPayload{Model: opts.Model, Messages: msgs, MaxTokens: opts.MaxTokens}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, newErr(ErrCodeSerializationFailed,
			"noop: failed to marshal request", err)
	}
	return data, nil
}
