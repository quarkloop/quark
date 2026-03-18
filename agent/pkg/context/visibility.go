package llmctx

import "sync"

// =============================================================================
// visibility.go
//
// Defines the Visibility bitmask, combinable presets, and the VisibilityPolicy
// that maps MessageType → default Visibility.
//
// Design:
//   - Visibility is a uint8 bitmask; three independent bits control three surfaces.
//   - VisibilityPolicy holds per-type overrides on top of package-level defaults.
//   - The defaultVisibility map is the single source of truth for defaults and
//     is also the reference used by the New*Message constructors in message.go.
// =============================================================================

// -----------------------------------------------------------------------------
// Visibility — bitmask
// -----------------------------------------------------------------------------

// Visibility is a bitmask controlling which surfaces a message is exposed to.
// Combine flags with bitwise OR:
//
//	v := VisibleToUser | VisibleToLLM  // visible in UI and LLM context
type Visibility uint8

const (
	// VisibleToUser surfaces the message in the end-user chat interface.
	VisibleToUser Visibility = 1 << iota // 0b001

	// VisibleToLLM includes the message in the token context sent to the LLM.
	VisibleToLLM // 0b010

	// VisibleToDeveloper surfaces the message in developer/debug tooling.
	VisibleToDeveloper // 0b100
)

// Preset combinations for common use cases.
var (
	// VisibleToAll shows the message on every surface.
	VisibleToAll = VisibleToUser | VisibleToLLM | VisibleToDeveloper

	// VisibleToLLMAndDev hides the message from users but keeps it in the LLM
	// context and developer tooling. Default for system prompts, memories, tool I/O.
	VisibleToLLMAndDev = VisibleToLLM | VisibleToDeveloper

	// VisibleToDeveloperOnly shows the message only in debug tooling.
	// Default for chain-of-thought and log messages.
	VisibleToDeveloperOnly = Visibility(VisibleToDeveloper)

	// VisibleToUserAndDev shows the message to the user and developer, but not
	// the LLM. Useful for UI-only annotations.
	VisibleToUserAndDev = VisibleToUser | VisibleToDeveloper

	// HiddenFromAll stores the message in history without surfacing it anywhere.
	HiddenFromAll = Visibility(0)
)

// HasFlag reports whether v includes the given flag bit.
func (v Visibility) HasFlag(flag Visibility) bool { return v&flag != 0 }

func (v Visibility) IsVisibleToUser() bool      { return v.HasFlag(VisibleToUser) }
func (v Visibility) IsVisibleToLLM() bool       { return v.HasFlag(VisibleToLLM) }
func (v Visibility) IsVisibleToDeveloper() bool { return v.HasFlag(VisibleToDeveloper) }

// String returns a human-readable description, e.g. "user|llm|dev".
func (v Visibility) String() string {
	var parts []string
	if v.IsVisibleToUser() {
		parts = append(parts, "user")
	}
	if v.IsVisibleToLLM() {
		parts = append(parts, "llm")
	}
	if v.IsVisibleToDeveloper() {
		parts = append(parts, "dev")
	}
	if len(parts) == 0 {
		return "hidden"
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += "|" + p
	}
	return result
}

// -----------------------------------------------------------------------------
// Package-level default visibility map
// -----------------------------------------------------------------------------

// defaultVisibility is the canonical source of default visibility for every
// MessageType. It is consumed by:
//   - New*Message constructors (message.go)
//   - VisibilityPolicy.For() fallback
//
// Override per-type for a specific agent via VisibilityPolicy.Set().
var defaultVisibility = map[MessageType]Visibility{
	// System prompt: injected into LLM context; hidden from UI (raw instructions).
	SystemPromptType: VisibleToLLMAndDev,

	// Text turns: fully visible; this is the main conversation surface.
	TextMessageType: VisibleToAll,

	// Images, PDFs, audio: visible everywhere so users see what they sent.
	ImageMessageType: VisibleToAll,
	PDFMessageType:   VisibleToAll,
	AudioMessageType: VisibleToAll,

	// Tool I/O: visible to LLM (required for function calling) and dev tooling,
	// but hidden from users by default (too noisy). Override if your UI
	// renders tool activity explicitly.
	ToolCallMessageType:   VisibleToLLMAndDev,
	ToolResultMessageType: VisibleToLLMAndDev,

	// Memory: injected into LLM context silently; users should not see raw memory.
	MemoryMessageType: VisibleToLLMAndDev,

	// Reasoning: dev-only by default. Override to VisibleToLLMAndDev for
	// models that benefit from explicit scratchpad turns.
	ReasoningMessageType: VisibleToDeveloperOnly,

	// Log: audit trail, never forwarded to users or LLM.
	LogMessageType: VisibleToDeveloperOnly,

	// Error: shown to both user and developer so the user knows something failed.
	ErrorMessageType: VisibleToUserAndDev,

	// Plan: shown everywhere; helps users understand what the agent is doing.
	PlanMessageType: VisibleToAll,
}

// -----------------------------------------------------------------------------
// VisibilityPolicy
// -----------------------------------------------------------------------------

// VisibilityPolicy maps MessageTypes to their effective Visibility.
//
// Safe for concurrent use from multiple goroutines: Set, Reset, For, and Clone
// may all be called simultaneously without external synchronisation. (R19)
//
// Start with DefaultVisibilityPolicy() and call Set() for any types that
// your application needs to display differently from the defaults.
//
// Example — show tool activity in the UI:
//
//	policy := llmctx.DefaultVisibilityPolicy()
//	policy.Set(llmctx.ToolCallMessageType,   llmctx.VisibleToAll)
//	policy.Set(llmctx.ToolResultMessageType, llmctx.VisibleToAll)
//
// Example — hide reasoning even from developers:
//
//	policy.Set(llmctx.ReasoningMessageType, llmctx.HiddenFromAll)
type VisibilityPolicy struct {
	mu        sync.RWMutex // R19: protects overrides from concurrent access
	overrides map[MessageType]Visibility
}

// DefaultVisibilityPolicy returns a policy backed by the package-level defaults.
// No overrides are set; call Set() to customise.
func DefaultVisibilityPolicy() *VisibilityPolicy {
	return &VisibilityPolicy{overrides: make(map[MessageType]Visibility)}
}

// Set registers a visibility override for msgType.
// Subsequent calls to For(msgType) return the overridden value.
// Safe for concurrent use.
func (p *VisibilityPolicy) Set(msgType MessageType, v Visibility) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.overrides[msgType] = v
}

// Reset removes a previously set override, reverting msgType to the package default.
// Safe for concurrent use.
func (p *VisibilityPolicy) Reset(msgType MessageType) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.overrides, msgType)
}

// For returns the effective Visibility for msgType.
// Priority: explicit override → package default → VisibleToDeveloperOnly (safe fallback).
// Safe for concurrent use.
func (p *VisibilityPolicy) For(msgType MessageType) Visibility {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if v, ok := p.overrides[msgType]; ok {
		return v
	}
	if v, ok := defaultVisibility[msgType]; ok {
		return v
	}
	return VisibleToDeveloperOnly // unknown types: dev-only is the safest default
}

// Clone returns an independent deep copy of the policy.
// Safe for concurrent use.
func (p *VisibilityPolicy) Clone() *VisibilityPolicy {
	p.mu.RLock()
	defer p.mu.RUnlock()
	np := DefaultVisibilityPolicy()
	for k, v := range p.overrides {
		np.overrides[k] = v
	}
	return np
}
