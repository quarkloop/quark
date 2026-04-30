package bash

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/pkg/toolkit"
)

var manifest *plugin.Manifest

func init() {
	var err error
	manifest, err = toolkit.LoadManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bash: %v\n", err)
		os.Exit(1)
	}
}

// Tool implements the bash AgentTool.
type Tool struct{}

func (t *Tool) Name() string {
	return manifest.Name
}

func (t *Tool) Version() string {
	return manifest.Version
}

func (t *Tool) Description() string {
	return manifest.Description
}

func (t *Tool) Schema() plugin.ToolSchema {
	if manifest.Tool != nil {
		return manifest.Tool.Schema
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
