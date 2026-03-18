// Package websearch implements the quark web-search tool — web search providers.
package websearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// SearchResult represents a single web search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Search performs a web search using the configured provider.
// Provider is selected via environment variables: BRAVE_API_KEY or SERPAPI_KEY.
// If neither is set, a stub response is returned.
func Search(query string, maxResults int) ([]SearchResult, error) {
	if key := os.Getenv("BRAVE_API_KEY"); key != "" {
		return searchBrave(query, maxResults, key)
	}
	if key := os.Getenv("SERPAPI_KEY"); key != "" {
		return searchSerpAPI(query, maxResults, key)
	}
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
