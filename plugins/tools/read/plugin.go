//go:build plugin

// Plugin export for lib-mode loading.
// Build with: go build -buildmode=plugin -tags plugin -o plugin.so ./
package main

import (
	"context"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/plugins/tools/read/pkg/read"
)

// QuarkPlugin is the exported plugin instance for lib-mode loading.
var QuarkPlugin plugin.Plugin = &ReadTool{}

// ReadTool implements the ToolPlugin interface for file reading.
type ReadTool struct{}

func (t *ReadTool) Name() string {
	return "read"
}

func (t *ReadTool) Version() string {
	return "1.0.0"
}

func (t *ReadTool) Type() plugin.PluginType {
	return plugin.TypeTool
}

func (t *ReadTool) Initialize(ctx context.Context, config map[string]any) error {
	return nil
}

func (t *ReadTool) Shutdown(ctx context.Context) error {
	return nil
}

func (t *ReadTool) Schema() plugin.ToolSchema {
	return plugin.ToolSchema{
		Name:        "read",
		Description: "Read a text file with optional line range",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read",
				},
				"start_line": map[string]any{
					"type":        "integer",
					"description": "Starting line number (1-based, optional)",
				},
				"end_line": map[string]any{
					"type":        "integer",
					"description": "Ending line number (1-based, optional)",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (t *ReadTool) Execute(ctx context.Context, args map[string]any) (plugin.ToolResult, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return plugin.ToolResult{
			IsError: true,
			Error:   "path is required",
		}, nil
	}

	req := read.Request{Path: path}
	if startLine, ok := args["start_line"].(float64); ok {
		req.StartLine = int(startLine)
	}
	if endLine, ok := args["end_line"].(float64); ok {
		req.EndLine = int(endLine)
	}

	result, err := read.Apply(req)
	if err != nil {
		return plugin.ToolResult{
			IsError: true,
			Error:   err.Error(),
		}, nil
	}

	return plugin.ToolResult{
		Output: map[string]any{
			"path":            result.Path,
			"content":         result.Content,
			"content_preview": result.ContentPreview,
			"file_size":       result.FileSize,
			"bytes_read":      result.BytesRead,
			"total_lines":     result.TotalLines,
			"start_line":      result.StartLine,
			"end_line":        result.EndLine,
		},
	}, nil
}
