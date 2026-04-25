package fs

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/pkg/toolkit"
)

var manifest *plugin.Manifest

func init() {
	var err error
	manifest, err = toolkit.LoadManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fs: %v\n", err)
		os.Exit(1)
	}
}

// Tool implements filesystem operations.
type Tool struct{}

// Name returns the tool name.
func (t *Tool) Name() string {
	return manifest.Name
}

// Version returns the tool version.
func (t *Tool) Version() string {
	return manifest.Version
}

// Description returns the tool description.
func (t *Tool) Description() string {
	return manifest.Description
}

// Schema returns the tool schema for LLM function calling.
func (t *Tool) Schema() plugin.ToolSchema {
	if manifest.Tool != nil {
		return manifest.Tool.Schema
	}
	return plugin.ToolSchema{
		Name:        "fs",
		Description: "Read, write, append, replace, list, stat, remove files and directories",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type": "string",
					"enum": []string{"read", "write", "append", "replace", "list", "stat", "rm"},
				},
				"path": map[string]any{
					"type": "string",
				},
				"content": map[string]any{
					"type": "string",
				},
				"find": map[string]any{
					"type": "string",
				},
				"replace_with": map[string]any{
					"type": "string",
				},
				"start_line": map[string]any{
					"type":        "integer",
					"description": "1-based start line for partial read",
				},
				"end_line": map[string]any{
					"type":        "integer",
					"description": "1-based inclusive end line for partial read",
				},
			},
			"required": []string{"command", "path"},
		},
	}
}

// Commands returns the available filesystem commands.
func (t *Tool) Commands() []toolkit.Command {
	return []toolkit.Command{
		{
			Name:        "read",
			Description: "Read a text file, optionally with line range",
			Args: []toolkit.Arg{
				{Name: "path", Description: "File path", Required: true},
			},
			Flags: []toolkit.Flag{
				{Name: "start-line", Type: "int", Description: "1-based start line (optional)", Default: 0},
				{Name: "end-line", Type: "int", Description: "1-based inclusive end line (optional)", Default: 0},
			},
			Handler: func(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
				return handleRead(input)
			},
		},
		{
			Name:        "write",
			Description: "Write content to a file (overwrite)",
			Args: []toolkit.Arg{
				{Name: "path", Description: "File path", Required: true},
				{Name: "content", Description: "Content to write", Required: true},
			},
			Handler: func(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
				err := os.WriteFile(input.Args["path"], []byte(input.Args["content"]), 0644)
				if err != nil {
					return toolkit.Output{Error: err.Error()}, nil
				}
				return toolkit.Output{Data: map[string]any{"written": len(input.Args["content"])}}, nil
			},
		},
		{
			Name:        "append",
			Description: "Append content to a file",
			Args: []toolkit.Arg{
				{Name: "path", Description: "File path", Required: true},
				{Name: "content", Description: "Content to append", Required: true},
			},
			Handler: func(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
				f, err := os.OpenFile(input.Args["path"], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					return toolkit.Output{Error: err.Error()}, nil
				}
				defer f.Close()
				n, err := f.WriteString(input.Args["content"])
				if err != nil {
					return toolkit.Output{Error: err.Error()}, nil
				}
				return toolkit.Output{Data: map[string]any{"appended": n}}, nil
			},
		},
		{
			Name:        "replace",
			Description: "Replace all occurrences of text in a file",
			Args: []toolkit.Arg{
				{Name: "path", Description: "File path", Required: true},
				{Name: "find", Description: "Text to find", Required: true},
				{Name: "replace-with", Description: "Replacement text", Required: true},
			},
			Handler: func(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
				data, err := os.ReadFile(input.Args["path"])
				if err != nil {
					return toolkit.Output{Error: err.Error()}, nil
				}
				replacements := strings.Count(string(data), input.Args["find"])
				newContent := strings.ReplaceAll(string(data), input.Args["find"], input.Args["replace-with"])
				if err := os.WriteFile(input.Args["path"], []byte(newContent), 0644); err != nil {
					return toolkit.Output{Error: err.Error()}, nil
				}
				return toolkit.Output{Data: map[string]any{"replacements": replacements}}, nil
			},
		},
		{
			Name:        "list",
			Description: "List directory contents",
			Args: []toolkit.Arg{
				{Name: "path", Description: "Directory path", Required: false, Default: "."},
			},
			Handler: func(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
				path := input.Args["path"]
				if path == "" {
					path = "."
				}
				entries, err := os.ReadDir(path)
				if err != nil {
					return toolkit.Output{Error: err.Error()}, nil
				}
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				return toolkit.Output{Data: map[string]any{"entries": names}}, nil
			},
		},
		{
			Name:        "stat",
			Description: "Get file metadata",
			Args: []toolkit.Arg{
				{Name: "path", Description: "File path", Required: true},
			},
			Handler: func(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
				info, err := os.Stat(input.Args["path"])
				if err != nil {
					return toolkit.Output{Error: err.Error()}, nil
				}
				return toolkit.Output{Data: map[string]any{
					"size":     info.Size(),
					"mode":     info.Mode().String(),
					"modified": info.ModTime(),
					"is_dir":   info.IsDir(),
				}}, nil
			},
		},
		{
			Name:        "rm",
			Description: "Remove a file or directory",
			Args: []toolkit.Arg{
				{Name: "path", Description: "File or directory path", Required: true},
			},
			Handler: func(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
				err := os.RemoveAll(input.Args["path"])
				if err != nil {
					return toolkit.Output{Error: err.Error()}, nil
				}
				return toolkit.Output{}, nil
			},
		},
	}
}

func handleRead(input toolkit.Input) (toolkit.Output, error) {
	path := input.Args["path"]
	data, err := os.ReadFile(path)
	if err != nil {
		return toolkit.Output{Error: err.Error()}, nil
	}
	content := string(data)
	startLine := 0
	endLine := 0
	if v, ok := input.Flags["start-line"]; ok {
		startLine = v.(int)
	}
	if v, ok := input.Flags["end-line"]; ok {
		endLine = v.(int)
	}
	if startLine > 0 || endLine > 0 {
		lines := strings.Split(content, "\n")
		total := len(lines)
		if startLine <= 0 {
			startLine = 1
		}
		if endLine <= 0 || endLine > total {
			endLine = total
		}
		if startLine > total {
			startLine = total
		}
		var selected []string
		for i := startLine - 1; i < endLine && i < total; i++ {
			selected = append(selected, lines[i])
		}
		content = strings.Join(selected, "\n")
		return toolkit.Output{Data: map[string]any{
			"content":     content,
			"total_lines": total,
			"start_line":  startLine,
			"end_line":    endLine,
		}}, nil
	}
	return toolkit.Output{Data: map[string]any{"content": content}}, nil
}
