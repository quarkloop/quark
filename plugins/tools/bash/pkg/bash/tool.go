package bash

import (
	"context"
	"os/exec"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/pkg/toolkit"
)

// Tool implements the bash AgentTool.
type Tool struct {
	manifest *plugin.Manifest
}

func (t *Tool) SetManifest(m *plugin.Manifest) {
	t.manifest = m
}

func (t *Tool) Name() string {
	if t.manifest == nil {
		return "bash"
	}
	return t.manifest.Name
}

func (t *Tool) Version() string {
	if t.manifest == nil {
		return "1.0.0"
	}
	return t.manifest.Version
}

func (t *Tool) Description() string {
	if t.manifest == nil {
		return "Shell command execution with sandboxing support"
	}
	return t.manifest.Description
}

func (t *Tool) Schema() plugin.ToolSchema {
	if t.manifest != nil && t.manifest.Tool != nil {
		return t.manifest.Tool.Schema
	}
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

func (t *Tool) Commands() []toolkit.Command {
	return []toolkit.Command{
		{
			Name:        "run",
			Description: "Execute a shell command",
			Args: []toolkit.Arg{
				{Name: "cmd", Description: "Shell command", Required: true},
			},
			Handler: func(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
				command := input.Args["cmd"]
				if command == "" {
					return toolkit.Output{Error: "cmd is required"}, nil
				}
				out, err := Execute(command)
				exitCode := 0
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						exitCode = exitErr.ExitCode()
					}
				}
				return toolkit.Output{Data: map[string]any{
					"output":    string(out),
					"exit_code": exitCode,
				}}, nil
			},
		},
	}
}
