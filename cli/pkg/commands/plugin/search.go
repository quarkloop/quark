package plugin

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type registryPlugin struct {
	Name        string
	Description string
}

func newSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search the plugin registry (github.com/quarkloop/plugins)",
		Long: `Search the plugin registry at github.com/quarkloop/plugins.

Downloads and parses the README.md to match plugin names and descriptions.
Falls back to the curated builtin plugin index if the README cannot be parsed.`,
		Args: cobra.ExactArgs(1),
		RunE: runSearch,
	}
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.ToLower(args[0])
	results, err := fetchRegistrySearch(query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		fmt.Println("No plugins found.")
		return nil
	}
	for _, r := range results {
		fmt.Printf("%-30s %s\n", r.Name, r.Description)
	}
	return nil
}

// pluginListRe matches markdown list items like:
// - tool-bash — Shell command execution
var pluginListRe = regexp.MustCompile(`[-*]\s+` + "`" + `([^` + "`" + `]+)` + "`" + `\s*[^[:space:]]+\s*(.*)`)

func fetchRegistrySearch(query string) ([]registryPlugin, error) {
	readmeURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/README.md",
		registryOwner, registryRepo)

	resp, err := httpGet(readmeURL)
	if err != nil {
		return builtinPlugins(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return builtinPlugins(), nil
	}

	plugins := parseRegistryREADME(resp.Body)

	var results []registryPlugin
	for _, p := range plugins {
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.Description), query) {
			results = append(results, p)
		}
	}

	if len(results) == 0 {
		return builtinPlugins(), nil
	}
	return results, nil
}

func parseRegistryREADME(body io.Reader) []registryPlugin {
	data, err := io.ReadAll(body)
	if err != nil {
		return builtinPlugins()
	}
	text := string(data)

	var plugins []registryPlugin
	for _, m := range pluginListRe.FindAllStringSubmatch(text, -1) {
		name := m[1]
		desc := strings.TrimSpace(m[2])
		plugins = append(plugins, registryPlugin{Name: name, Description: desc})
	}

	if len(plugins) == 0 {
		return builtinPlugins()
	}
	return plugins
}

func builtinPlugins() []registryPlugin {
	return []registryPlugin{
		{Name: "tool-bash", Description: "Shell command execution with sandboxing support"},
		{Name: "tool-read", Description: "Read regular text files with line range support"},
		{Name: "tool-write", Description: "Write and edit regular text files"},
		{Name: "tool-web-search", Description: "Web search via Brave/SerpAPI with stub fallback"},
	}
}

func httpGet(url string) (*http.Response, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	return client.Get(url)
}
