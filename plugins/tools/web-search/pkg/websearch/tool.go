package websearch

import (
	"context"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
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
	}
}

// searchHandler returns a Fiber handler for the search endpoint.
func searchHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(`{"error":"invalid request"}`)
		}
		if req.Query == "" {
			return c.Status(fiber.StatusBadRequest).SendString(`{"error":"query is required"}`)
		}
		if req.MaxResults == 0 {
			req.MaxResults = 5
		}
		results, err := Search(req.Query, req.MaxResults)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf(`{"error":%q}`, err.Error()))
		}
		return c.JSON(fiber.Map{"results": results})
	}
}

// Serve starts an HTTP server for the web-search tool on the given address (api mode).
func Serve(addr string) error {
	app := fiber.New()
	app.Post("/search", searchHandler())
	fmt.Printf("web-search tool listening on %s\n", addr)
	return app.Listen(addr)
}
