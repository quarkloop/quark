// Package rendering implements the "visibility as rendering target" pattern.
//
// # Concept
//
// The current design uses a visibility bitmask to decide *who sees* a message.
// This package extends that: each surface receives a *different representation*
// of the same message, tailored to its audience.
//
//   - LLM surface:       dense, token-efficient, model-appropriate text
//   - User surface:      friendly prose; hides internal mechanics
//   - Developer surface: full structural detail, labels, metadata
//
// A ToolCallMessage does not just get filtered in/out — it renders as
// "Tool call: get_weather({…})" for the model, "🔧 Checking weather…" for the
// user, and "[tool_call id=x1 name=get_weather]\nargs: {…}" for the developer.
//
// # Integration
//
// All concrete Payload types in the message sub-package implement the Renderer
// interface via their LLMText, UserText, and DevText methods.
//
// Call RenderFor(surface, payload) to dispatch to the right method.
// The payload parameter is typed as Renderer so this package has no import
// dependency on the parent llmctx package or the message sub-package.
package rendering

// =============================================================================
// Surface
// =============================================================================

// Surface identifies the rendering target audience.
type Surface int

const (
	// SurfaceLLM produces text for the model's context window.
	SurfaceLLM Surface = iota
	// SurfaceUser produces text for the end-user chat interface.
	SurfaceUser
	// SurfaceDeveloper produces text for developer/debug tooling.
	SurfaceDeveloper
)

// String returns the name of the surface.
func (s Surface) String() string {
	switch s {
	case SurfaceLLM:
		return "llm"
	case SurfaceUser:
		return "user"
	case SurfaceDeveloper:
		return "developer"
	default:
		return "unknown"
	}
}

// =============================================================================
// Renderer interface
// =============================================================================

// Renderer is implemented by every Payload type in the message package.
// Each method returns the surface-appropriate string for that payload.
//
// A return value of "" means "do not surface this message to this audience".
//
// All built-in payload types (TextPayload, ToolCallPayload, etc.) implement
// this interface via their LLMText / UserText / DevText methods.
type Renderer interface {
	// LLMText is the string injected into the LLM context window.
	LLMText() string
	// UserText is the string shown in the end-user chat interface.
	UserText() string
	// DevText is the string shown in developer/debug tooling.
	DevText() string
	// TextRepresentation is the canonical flat-string form (for fallback).
	TextRepresentation() string
}

// =============================================================================
// RenderFor
// =============================================================================

// RenderFor dispatches to the appropriate render method on r.
// Returns "" when the renderer has no content for surface.
func RenderFor(surface Surface, r Renderer) string {
	switch surface {
	case SurfaceLLM:
		return r.LLMText()
	case SurfaceUser:
		return r.UserText()
	case SurfaceDeveloper:
		return r.DevText()
	default:
		return r.DevText()
	}
}

// IsVisibleOn reports whether r has any content for surface.
func IsVisibleOn(surface Surface, r Renderer) bool {
	return RenderFor(surface, r) != ""
}

// =============================================================================
// FallbackRenderer — wraps any type that only has TextRepresentation
// =============================================================================

// FallbackRenderer adapts a TextRepresentation-only payload to the Renderer
// interface.  All surfaces return the same flat text.
// Used for payloads defined outside this package that haven't yet added the
// three surface methods.
type FallbackRenderer struct {
	// Text is the flat representation used for all surfaces.
	Text string
}

func (f FallbackRenderer) LLMText() string            { return f.Text }
func (f FallbackRenderer) UserText() string           { return f.Text }
func (f FallbackRenderer) DevText() string            { return f.Text }
func (f FallbackRenderer) TextRepresentation() string { return f.Text }

// =============================================================================
// RenderedMessage — a pre-rendered snapshot of all surfaces
// =============================================================================

// RenderedMessage caches the three surface strings for a single message,
// avoiding repeated method dispatch when a message needs to be rendered
// multiple times in one request cycle.
type RenderedMessage struct {
	MessageID string
	LLM       string
	User      string
	Developer string
}

// ForSurface returns the cached string for the given surface.
func (rm RenderedMessage) ForSurface(s Surface) string {
	switch s {
	case SurfaceLLM:
		return rm.LLM
	case SurfaceUser:
		return rm.User
	default:
		return rm.Developer
	}
}

// IsVisible reports whether the rendered message has any content for surface.
func (rm RenderedMessage) IsVisible(s Surface) bool {
	return rm.ForSurface(s) != ""
}

// Render produces a RenderedMessage by calling all three surface methods.
func Render(id string, r Renderer) RenderedMessage {
	return RenderedMessage{
		MessageID: id,
		LLM:       r.LLMText(),
		User:      r.UserText(),
		Developer: r.DevText(),
	}
}

