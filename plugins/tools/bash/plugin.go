//go:build plugin

// Plugin export for lib-mode loading.
// Build with: go build -buildmode=plugin -tags plugin -o plugin.so ./
package main

import (
	"context"
	"os/exec"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/plugins/tools/bash/pkg/bash"
)

// QuarkPlugin is the exported plugin instance for lib-mode loading.
var QuarkPlugin plugin.Plugin = &BashTool{}

// BashTool implements the ToolPlugin interface for shell command execution.
type BashTool struct{}

func (t *BashTool) Name() string {
	return "bash"
}

func (t *BashTool) Version() string {
	return "1.0.0"
}

func (t *BashTool) Type() plugin.PluginType {
	return plugin.TypeTool
}

func (t *BashTool) Initialize(ctx context.Context, config map[string]any) error {
	return nil
}

func (t *BashTool) Shutdown(ctx context.Context) error {
	return nil
}

func (t *BashTool) Schema() plugin.ToolSchema {
	return plugin.ToolSchema{
		Name:        "bash",
		Description: "Execute a shell command and return output",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cmd": map[string]any{
					"type":        "string",
					"description": "Shell command to execute",
				},
			},
			"required": []string{"cmd"},
		},
	}
}

func (t *BashTool) Execute(ctx context.Context, args map[string]any) (plugin.ToolResult, error) {
	cmd, ok := args["cmd"].(string)
	if !ok || cmd == "" {
		return plugin.ToolResult{
			IsError: true,
			Error:   "cmd is required",
		}, nil
	}

	output, err := bash.Execute(cmd)
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return plugin.ToolResult{
		Output: map[string]any{
			"output":    string(output),
			"exit_code": exitCode,
		},
	}, nil
}
