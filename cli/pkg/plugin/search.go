package plugin

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// RegistryPlugin represents a plugin listing from the registry.
type RegistryPlugin struct {
	Name        string
	Description string
}

var pluginListRe = regexp.MustCompile(`[-*]\s+` + "`" + `([^` + "`" + `]+)` + "`" + `\s*[^[:space:]]+\s*(.*)`)

// Search searches the plugin registry for entries matching query.
func Search(query string) ([]RegistryPlugin, error) {
	query = strings.ToLower(query)
	readmeURL := rawReadmeURL()

	resp, err := httpGet(readmeURL)
	if err != nil {
		return BuiltinPlugins(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return BuiltinPlugins(), nil
	}

	results := parseRegistryREADME(resp.Body, query)
	if len(results) == 0 {
		return BuiltinPlugins(), nil
	}
	return results, nil
}

func parseRegistryREADME(body io.Reader, query string) (results []RegistryPlugin) {
	data, err := io.ReadAll(body)
	if err != nil {
		return
	}
	text := string(data)
	for _, m := range pluginListRe.FindAllStringSubmatch(text, -1) {
		name := m[1]
		desc := strings.TrimSpace(m[2])
		if strings.Contains(strings.ToLower(name), query) ||
			strings.Contains(strings.ToLower(desc), query) {
			results = append(results, RegistryPlugin{Name: name, Description: desc})
		}
	}
	return
}

// BuiltinPlugins returns the known builtin plugins.
func BuiltinPlugins() []RegistryPlugin {
	return []RegistryPlugin{
		{Name: "tool-bash", Description: "Shell command execution with sandboxing support"},
		{Name: "tool-read", Description: "Read regular text files with line range support"},
		{Name: "tool-write", Description: "Write and edit regular text files"},
		{Name: "tool-web-search", Description: "Web search via Brave/SerpAPI with stub fallback"},
	}
}

func rawReadmeURL() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/README.md", registryOwner, registryRepo)
}

func httpGet(url string) (*http.Response, error) {
	c := &http.Client{Timeout: 15 * time.Second}
	return c.Get(url)
}
