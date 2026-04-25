//go:build plugin

// Plugin export for lib-mode loading.
// Build with: go build -buildmode=plugin -tags plugin -o plugin.so ./
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/pkg/toolkit"
	"github.com/quarkloop/plugins/tools/web-search/pkg/websearch"
)

var (
	manifest *plugin.Manifest
)

func init() {
	var err error
	manifest, err = toolkit.LoadManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "web-search: %v\n", err)
		os.Exit(1)
	}
}

// QuarkPlugin is the exported plugin instance for lib-mode loading.
var QuarkPlugin plugin.Plugin = &WebSearchTool{}

// WebSearchTool implements the ToolPlugin interface, sourcing metadata from manifest.yaml.
type WebSearchTool struct{}

func (t *WebSearchTool) Name() string    { return manifest.Name }
func (t *WebSearchTool) Version() string { return manifest.Version }
func (t *WebSearchTool) Type() plugin.PluginType { return manifest.Type }
func (t *WebSearchTool) Initialize(ctx context.Context, cfg map[string]any) error { return nil }
func (t *WebSearchTool) Shutdown(ctx context.Context) error { return nil }

func (t *WebSearchTool) Schema() plugin.ToolSchema {
	if manifest.Tool != nil {
		return manifest.Tool.Schema
	}
	return plugin.ToolSchema{
		Name:        manifest.Name,
		Description: manifest.Description,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"q": map[string]any{
					"type":        "string",
					"description": "Search query",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results (default 10)",
				},
			},
			"required": []string{"q"},
		},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) (plugin.ToolResult, error) {
	query, _ := args["q"].(string)
	if query == "" {
		return plugin.ToolResult{IsError: true, Error: "q (query) is required"}, nil
	}

	maxResults := 10
	if mr, ok := args["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	results, err := websearch.Search(query, maxResults)
	if err != nil {
		return plugin.ToolResult{IsError: true, Error: err.Error()}, nil
	}

	resultsAny := make([]any, len(results))
	for i, r := range results {
		resultsAny[i] = map[string]any{
			"title":   r.Title,
			"url":     r.URL,
			"snippet": r.Snippet,
		}
	}

	return plugin.ToolResult{
		Output: map[string]any{
			"query":   query,
			"results": resultsAny,
			"count":   len(results),
		},
	}, nil
}
