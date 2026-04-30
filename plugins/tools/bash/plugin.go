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

// NewPlugin creates a new BashTool instance for lib-mode loading.
func NewPlugin() plugin.Plugin {
	return &BashTool{}
}

// BashTool implements the ToolPlugin interface, sourcing metadata from manifest.yaml.
type BashTool struct {
	manifest *plugin.Manifest
}

func (t *BashTool) Name() string    { return t.manifest.Name }
func (t *BashTool) Version() string { return t.manifest.Version }
func (t *BashTool) Type() plugin.PluginType { return t.manifest.Type }

func (t *BashTool) Initialize(ctx context.Context, cfg map[string]any) error {
	manifest, err := plugin.ParseManifest("manifest.yaml")
	if err != nil {
		return err
	}
	t.manifest = manifest
	return nil
}

func (t *BashTool) Shutdown(ctx context.Context) error { return nil }

func (t *BashTool) Schema() plugin.ToolSchema {
	if t.manifest.Tool != nil {
		return t.manifest.Tool.Schema
	}
	return plugin.ToolSchema{
		Name:        t.manifest.Name,
		Description: t.manifest.Description,
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
		return plugin.ToolResult{IsError: true, Error: "cmd is required"}, nil
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
