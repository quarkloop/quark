package pluginmanager

import (
	"context"

	"github.com/quarkloop/pkg/plugin"
)

type RuntimeToolHandler func(ctx context.Context, arguments string) (string, error)

type RuntimeTool struct {
	Schema  plugin.ToolSchema
	Handler RuntimeToolHandler
}

func (m *Manager) RegisterRuntimeTool(tool RuntimeTool) {
	if tool.Schema.Name == "" || tool.Handler == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.runtimeTools[tool.Schema.Name] = tool.Handler
	m.removeToolSchemaLocked(tool.Schema.Name)
	m.tools = append(m.tools, tool.Schema)
}
