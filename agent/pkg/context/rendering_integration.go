package llmctx

import (
	"github.com/quarkloop/agent/pkg/context/rendering"
)

// =============================================================================
// rendering_integration.go  —  Surface-aware rendering on AgentContext
//
// The rendering package defines three surfaces: LLM, User, Developer.
// Each Payload type implements LLMText / UserText / DevText to provide the
// appropriate representation for each audience.
//
// This file wires those surfaces into the AgentContext layer so callers
// building custom adapters, UI layers, or debug tooling can request exactly
// the representation they need without digging into the Payload interface.
// =============================================================================

// Surface re-exports the rendering.Surface type so callers can use
// llmctx.SurfaceLLM without importing the rendering sub-package.
type Surface = rendering.Surface

// Surface constants — re-exported for convenience.
const (
	// SurfaceLLM selects the token-efficient model-appropriate representation.
	SurfaceLLM = rendering.SurfaceLLM
	// SurfaceUser selects the friendly end-user prose representation.
	SurfaceUser = rendering.SurfaceUser
	// SurfaceDeveloper selects the full structural detail representation.
	SurfaceDeveloper = rendering.SurfaceDeveloper
)

// RenderMessage returns the string representation of a single Message for the
// given surface.  Returns "" when the payload has no content for that surface
// (e.g. a LogPayload returns "" for SurfaceLLM and SurfaceUser).
//
// Example:
//
//	text := llmctx.RenderMessage(msg, llmctx.SurfaceUser)
func RenderMessage(m *Message, surface Surface) string {
	return rendering.RenderFor(surface, m.payload)
}

// RenderedMessage is a pre-rendered snapshot of a message across all three
// surfaces.  Building it once avoids repeated interface dispatch when the
// same message is rendered multiple times in one request cycle.
type RenderedMessage = rendering.RenderedMessage

// RenderAllSurfaces produces a RenderedMessage for m.
//
// Example:
//
//	rm := llmctx.RenderAllSurfaces(msg)
//	llmText  := rm.LLM
//	userText := rm.User
//	devText  := rm.Developer
func RenderAllSurfaces(m *Message) RenderedMessage {
	return rendering.Render(m.ID().String(), m.payload)
}

// RenderedMessages returns a RenderedMessage for every message visible on
// the given surface, in insertion order.  Messages with no content on the
// surface are excluded (their rendered string is "").
//
// Use this to build the payload for a UI layer or debug panel without
// writing adapter boilerplate:
//
//	// Build the user-facing conversation transcript.
//	rendered := ac.RenderedMessages(llmctx.SurfaceUser)
//	for _, rm := range rendered {
//	    fmt.Println(rm.User)
//	}
func (ac *AgentContext) RenderedMessages(surface Surface) []RenderedMessage {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	out := make([]RenderedMessage, 0, len(ac.messages))
	for _, m := range ac.messages {
		rm := rendering.Render(m.ID().String(), m.payload)
		if rm.IsVisible(surface) {
			out = append(out, rm)
		}
	}
	return out
}

// RenderContext returns all three surface representations for every message
// in the context, regardless of visibility.  Useful for developer tooling
// that needs to inspect the full picture.
//
// Returns a map: messageID → RenderedMessage.
func (ac *AgentContext) RenderContext() map[string]RenderedMessage {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	out := make(map[string]RenderedMessage, len(ac.messages))
	for _, m := range ac.messages {
		out[m.ID().String()] = rendering.Render(m.ID().String(), m.payload)
	}
	return out
}
