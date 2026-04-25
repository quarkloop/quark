package toolkit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// RunPipe reads one JSON object from stdin, executes the matching command,
// and writes the Output as JSON to stdout.
func RunPipe(tool AgentTool) error {
	var req struct {
		Command string            `json:"command"`
		Args    map[string]string `json:"args"`
		Flags   map[string]any    `json:"flags"`
	}

	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		return fmt.Errorf("parse stdin: %w", err)
	}

	cmd, ok := findCommand(tool, req.Command)
	if !ok {
		return json.NewEncoder(os.Stdout).Encode(Output{
			Error: "unknown command: " + req.Command,
		})
	}

	// Fill missing args with defaults
	input := Input{Args: make(map[string]string), Flags: make(map[string]any)}
	for _, arg := range cmd.Args {
		if v, ok := req.Args[arg.Name]; ok {
			input.Args[arg.Name] = v
		} else {
			input.Args[arg.Name] = arg.Default
		}
	}
	for k, v := range req.Flags {
		input.Flags[k] = v
	}

	out, err := cmd.Handler(context.Background(), input)
	if err != nil {
		out.Error = err.Error()
	}

	return json.NewEncoder(os.Stdout).Encode(out)
}
