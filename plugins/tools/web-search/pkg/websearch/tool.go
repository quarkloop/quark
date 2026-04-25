package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/pkg/toolkit"
)

var manifest *plugin.Manifest

func init() {
	var err error
	manifest, err = toolkit.LoadManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "web-search: %v\n", err)
		os.Exit(1)
	}
}

// Tool implements the web-search AgentTool.
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

func (t *Tool) Commands() []toolkit.Command {
	return []toolkit.Command{
		{
			Name:        "run",
			Description: "Search the web and return results",
			Args: []toolkit.Arg{
				{Name: "query", Description: "Search query", Required: true},
			},
			Flags: []toolkit.Flag{
				{Name: "max-results", Type: "int", Description: "Max results", Default: 10},
			},
			Handler: func(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
				query := input.Args["query"]
				if query == "" {
					return toolkit.Output{Error: "query is required"}, nil
				}
				maxResults := 10
				if mr, ok := input.Flags["max-results"].(int); ok && mr > 0 {
					maxResults = mr
				}
				results, err := Search(query, maxResults)
				if err != nil {
					return toolkit.Output{Error: err.Error()}, nil
				}
				resultsAny := make([]any, len(results))
				for i, r := range results {
					resultsAny[i] = map[string]any{
						"title":   r.Title,
						"url":     r.URL,
						"snippet": r.Snippet,
					}
				}
				return toolkit.Output{Data: map[string]any{
					"query":   query,
					"results": resultsAny,
					"count":   len(results),
				}}, nil
			},
		},
		{
			Name:        "serve",
			Description: "Start HTTP server",
			Args: []toolkit.Arg{
				{Name: "addr", Description: "Listen address", Required: false, Default: "127.0.0.1:8090"},
			},
			Handler: func(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
				addr := input.Args["addr"]
				if addr == "" {
					addr = "127.0.0.1:8090"
				}
				fmt.Printf("web-search tool listening on %s\n", addr)
				return toolkit.Output{}, Serve(addr)
			},
		},
	}
}

// Serve starts an HTTP server for the web-search tool on the given address.
func Serve(addr string) error {
	http.HandleFunc("POST /search", searchHandler())
	fmt.Printf("web-search tool listening on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}

func searchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		if req.Query == "" {
			http.Error(w, `{"error":"query is required"}`, http.StatusBadRequest)
			return
		}
		if req.MaxResults == 0 {
			req.MaxResults = 5
		}
		results, err := Search(req.Query, req.MaxResults)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": results,
		})
	}
}
