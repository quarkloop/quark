package agent

import (
	"context"

	"github.com/quarkloop/pkg/plugin"
)

// CapabilityProvider contributes non-plugin runtime capabilities to the agent.
// Implementations own their discovery, prompt text, tool schemas, and dispatch.
type CapabilityProvider interface {
	Prompt() string
	ToolSchemas() []plugin.ToolSchema
	ExecuteTool(ctx context.Context, name, arguments string) (output string, handled bool, err error)
}
