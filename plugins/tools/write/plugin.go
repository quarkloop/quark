//go:build plugin

// Plugin export for lib-mode loading.
// Build with: go build -buildmode=plugin -tags plugin -o plugin.so ./
package main

import (
	"context"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/plugins/tools/write/pkg/write"
)

// QuarkPlugin is the exported plugin instance for lib-mode loading.
var QuarkPlugin plugin.Plugin = &WriteTool{}

// WriteTool implements the ToolPlugin interface for file writing.
type WriteTool struct{}

func (t *WriteTool) Name() string {
	return "write"
}

func (t *WriteTool) Version() string {
	return "1.0.0"
}

func (t *WriteTool) Type() plugin.PluginType {
	return plugin.TypeTool
}

func (t *WriteTool) Initialize(ctx context.Context, config map[string]any) error {
	return nil
}

func (t *WriteTool) Shutdown(ctx context.Context) error {
	return nil
}

func (t *WriteTool) Schema() plugin.ToolSchema {
	return plugin.ToolSchema{
		Name:        "write",
		Description: "Write, append, replace, or edit text files",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file",
				},
				"operation": map[string]any{
					"type":        "string",
					"enum":        []string{"write", "append", "replace", "edit"},
					"description": "Operation type (write, append, replace, edit)",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write (for write/append operations)",
				},
				"find": map[string]any{
					"type":        "string",
					"description": "Text to find (for replace operation)",
				},
				"replace_with": map[string]any{
					"type":        "string",
					"description": "Replacement text (for replace operation)",
				},
			},
			"required": []string{"path", "operation"},
		},
	}
}

func (t *WriteTool) Execute(ctx context.Context, args map[string]any) (plugin.ToolResult, error) {
	path, _ := args["path"].(string)
	operation, _ := args["operation"].(string)

	if path == "" {
		return plugin.ToolResult{IsError: true, Error: "path is required"}, nil
	}
	if operation == "" {
		return plugin.ToolResult{IsError: true, Error: "operation is required"}, nil
	}

	req := write.Request{
		Path:      path,
		Operation: operation,
	}

	if content, ok := args["content"].(string); ok {
		req.Content = content
	}
	if find, ok := args["find"].(string); ok {
		req.Find = find
	}
	if replaceWith, ok := args["replace_with"].(string); ok {
		req.ReplaceWith = replaceWith
	}

	result, err := write.Apply(req)
	if err != nil {
		return plugin.ToolResult{IsError: true, Error: err.Error()}, nil
	}

	return plugin.ToolResult{
		Output: map[string]any{
			"path":            result.Path,
			"operation":       result.Operation,
			"created":         result.Created,
			"changed":         result.Changed,
			"bytes_written":   result.BytesWritten,
			"file_size":       result.FileSize,
			"content_preview": result.ContentPreview,
		},
	}, nil
}
