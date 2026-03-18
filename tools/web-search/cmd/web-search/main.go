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
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:          "web-search",
		Short:        "quark web-search tool — search the web",
		SilenceUsage: true,
	}

	root.AddCommand(runCmd())
	root.AddCommand(serveCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
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
			results, err := search(query, maxResults)
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
			http.HandleFunc("POST /search", func(w http.ResponseWriter, r *http.Request) {
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
				results, err := search(req.Query, req.MaxResults)
				if err != nil {
					http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"results": results,
				})
			})
			fmt.Printf("web-search tool listening on %s\n", addr)
			return http.ListenAndServe(addr, nil)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8090", "Address to listen on")
	return cmd
}

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func search(query string, maxResults int) ([]SearchResult, error) {
	if key := os.Getenv("BRAVE_API_KEY"); key != "" {
		return searchBrave(query, maxResults, key)
	}
	if key := os.Getenv("SERPAPI_KEY"); key != "" {
		return searchSerpAPI(query, maxResults, key)
	}
	// Stub: no provider configured.
	return []SearchResult{
		{
			Title:   fmt.Sprintf("No search provider configured for: %s", query),
			URL:     "https://example.com",
			Snippet: "Set BRAVE_API_KEY or SERPAPI_KEY to enable real search results.",
		},
	}, nil
}

func searchBrave(query string, maxResults int, apiKey string) ([]SearchResult, error) {
	reqURL := fmt.Sprintf(
		"https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), maxResults,
	)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave search: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("brave search: parse: %w", err)
	}

	out := make([]SearchResult, 0, len(result.Web.Results))
	for _, r := range result.Web.Results {
		out = append(out, SearchResult{Title: r.Title, URL: r.URL, Snippet: r.Description})
	}
	return out, nil
}

func searchSerpAPI(query string, maxResults int, apiKey string) ([]SearchResult, error) {
	reqURL := fmt.Sprintf(
		"https://serpapi.com/search.json?q=%s&num=%d&api_key=%s",
		url.QueryEscape(query), maxResults, apiKey,
	)
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("serpapi: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("serpapi: parse: %w", err)
	}

	out := make([]SearchResult, 0, len(result.OrganicResults))
	for _, r := range result.OrganicResults {
		out = append(out, SearchResult{Title: r.Title, URL: r.Link, Snippet: r.Snippet})
	}
	return out, nil
}
