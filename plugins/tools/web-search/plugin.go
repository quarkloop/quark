//go:build plugin

// Plugin export for lib-mode loading.
// Build with: go build -buildmode=plugin -tags plugin -o plugin.so ./
package main

import (
	"context"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/plugins/tools/web-search/pkg/websearch"
)

// QuarkPlugin is the exported plugin instance for lib-mode loading.
var QuarkPlugin plugin.Plugin = &WebSearchTool{}

// WebSearchTool implements the ToolPlugin interface for web search.
type WebSearchTool struct{}

func (t *WebSearchTool) Name() string {
	return "web-search"
}

func (t *WebSearchTool) Version() string {
	return "1.0.0"
}

func (t *WebSearchTool) Type() plugin.PluginType {
	return plugin.TypeTool
}

func (t *WebSearchTool) Initialize(ctx context.Context, config map[string]any) error {
	return nil
}

func (t *WebSearchTool) Shutdown(ctx context.Context) error {
	return nil
}

func (t *WebSearchTool) Schema() plugin.ToolSchema {
	return plugin.ToolSchema{
		Name:        "search",
		Description: "Search the web using Brave or SerpAPI",
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

	// Convert to []any for JSON serialization
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
