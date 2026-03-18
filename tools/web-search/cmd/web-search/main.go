// web-search is a quark tool that performs web searches.
//
// Usage as a tool:
//
//	web-search run --query "golang concurrency"
//
// Usage as an HTTP skill server (compatible with the skill dispatcher):
//
//	web-search serve --addr 127.0.0.1:8090
//
// The HTTP server accepts POST /search with body {"query":"...","max_results":10}
// and returns {"results":[{"title":"...","url":"...","snippet":"..."}]}.
//
// The upstream search provider is configured via environment variables:
//
//	BRAVE_API_KEY   — Brave Search API (https://api.search.brave.com)
//	SERPAPI_KEY     — SerpAPI (https://serpapi.com)
//
// If neither key is set, the server returns a stub response useful for testing.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/quarkloop/core/pkg/toolkit"
	"github.com/quarkloop/tools/web-search/pkg/websearch"
)

func main() {
	root := toolkit.NewToolCommand("web-search", "search the web")

	root.AddCommand(runCmd())
	root.AddCommand(serveCmd())

	toolkit.Execute(root)
}

func runCmd() *cobra.Command {
	var query string
	var maxResults int
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Search the web and print results as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				return fmt.Errorf("--query is required")
			}
			results, err := websearch.Search(query, maxResults)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"results": results,
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Search query")
	cmd.Flags().IntVar(&maxResults, "max-results", 5, "Maximum number of results")
	return cmd
}

func serveCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start an HTTP server that handles search requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			http.HandleFunc("POST /search", searchHandler())
			fmt.Printf("web-search tool listening on %s\n", addr)
			return http.ListenAndServe(addr, nil)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8090", "Address to listen on")
	return cmd
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
		results, err := websearch.Search(req.Query, req.MaxResults)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": results,
		})
	}
}
