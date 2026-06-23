package cmd

import (
        "encoding/json"
        "fmt"
        "io"
        "net/http"
        "os"
        "path/filepath"
        "net/url"
	"strings"

        "github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
        Use:   "node",
        Short: "Manage node packages in the registry",
        Long: `Manage node packages in the Quark Catalog registry.

Similar to 'docker image', these commands let you publish, pull,
list, and search for node implementations.

Examples:
  quarkctl node list
  quarkctl node info source/timer:v1
  quarkctl node search "cpu"
  quarkctl node push ./my-node.ts --uri function/my-node:v1
  quarkctl node pull source/timer:v1`,
}

var nodePushURI string
var nodePushCategory string
var nodePushVersion string
var nodePushType string

func init() {
        rootCmd.AddCommand(nodeCmd)
        nodeCmd.AddCommand(nodeListCmd)
        nodeCmd.AddCommand(nodeInfoCmd)
        nodeCmd.AddCommand(nodeSearchCmd)
        nodeCmd.AddCommand(nodePushCmd)
        nodeCmd.AddCommand(nodePullCmd)

        nodePushCmd.Flags().StringVar(&nodePushURI, "uri", "", "Node URI (e.g., function/my-node:v1)")
        nodePushCmd.Flags().StringVar(&nodePushCategory, "category", "function", "Node category (source|function|store|endpoint|policy)")
        nodePushCmd.Flags().StringVar(&nodePushVersion, "version", "1.0.0", "Node version")
        nodePushCmd.Flags().StringVar(&nodePushType, "type", "typescript", "Content type (typescript|shared-library)")
        _ = nodePushCmd.MarkFlagRequired("uri")
}

// node list
var nodeListCmd = &cobra.Command{
        Use:   "list",
        Short: "List available nodes in the registry",
        Args:  cobra.NoArgs,
        RunE:  runNodeList,
}

func runNodeList(cmd *cobra.Command, args []string) error {
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()

        resp, err := c.RawGet(ctx, "/api/v1/registry/nodes")
        if err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to list nodes: %w", err))
        }
        defer resp.Body.Close()

        var wrapper struct {
                Nodes []map[string]interface{} `json:"nodes"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to decode response: %w", err))
        }

        if flagJSON || flagOutput == "json" {
                return newPrinter().PrintRaw(wrapper.Nodes)
        }

        if len(wrapper.Nodes) == 0 {
                fmt.Fprintln(os.Stdout, "No nodes found in registry.")
                return nil
        }

        fmt.Fprintf(os.Stdout, "%-35s %-12s %-10s %-15s %s\n", "URI", "CATEGORY", "TYPE", "VERSION", "DESCRIPTION")
        for _, n := range wrapper.Nodes {
                uri, _ := n["uri"].(string)
                cat, _ := n["category"].(string)
                ct, _ := n["contentType"].(string)
                ver, _ := n["version"].(string)
                manifest, _ := n["manifest"].(string)
                desc := ""
                if manifest != "" {
                        var m map[string]interface{}
                        if json.Unmarshal([]byte(manifest), &m) == nil {
                                if d, ok := m["description"].(string); ok {
                                        desc = d
                                }
                        }
                }
                fmt.Fprintf(os.Stdout, "%-35s %-12s %-10s %-15s %s\n", uri, cat, ct, ver, desc)
        }
        return nil
}

// node info
var nodeInfoCmd = &cobra.Command{
        Use:   "info <uri>",
        Short: "Show details of a node package",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodeInfo,
}

func runNodeInfo(cmd *cobra.Command, args []string) error {
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()

        uri := args[0]
        resp, err := c.RawPost(ctx, "/api/v1/registry/nodes/info", map[string]string{"uri": uri})
        if err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to get node info: %w", err))
        }
        defer resp.Body.Close()

        var info map[string]interface{}
        if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to decode response: %w", err))
        }

        if flagJSON || flagOutput == "json" {
                return newPrinter().PrintRaw(info)
        }

        fmt.Fprintf(os.Stdout, "URI:          %s\n", info["uri"])
        fmt.Fprintf(os.Stdout, "Category:     %s\n", info["category"])
        fmt.Fprintf(os.Stdout, "Version:      %s\n", info["version"])
        fmt.Fprintf(os.Stdout, "Content Type: %s\n", info["contentType"])
        fmt.Fprintf(os.Stdout, "Checksum:     %s\n", info["checksum"])
        fmt.Fprintf(os.Stdout, "Created:      %s\n", info["createdAt"])
        fmt.Fprintf(os.Stdout, "Downloads:    %v\n", info["downloads"])
        if manifest, ok := info["manifest"].(string); ok && manifest != "" {
                fmt.Fprintf(os.Stdout, "\nManifest:\n%s\n", manifest)
        }
        return nil
}

// node search
var nodeSearchCmd = &cobra.Command{
        Use:   "search <keyword>",
        Short: "Search nodes by name or description",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodeSearch,
}

func runNodeSearch(cmd *cobra.Command, args []string) error {
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()

        keyword := args[0]
        resp, err := c.RawGet(ctx, "/api/v1/registry/nodes/search?keyword="+url.QueryEscape(keyword))
        if err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to search nodes: %w", err))
        }
        defer resp.Body.Close()

        var wrapper struct {
                Nodes []map[string]interface{} `json:"nodes"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to decode response: %w", err))
        }

        if flagJSON || flagOutput == "json" {
                return newPrinter().PrintRaw(wrapper.Nodes)
        }

        if len(wrapper.Nodes) == 0 {
                fmt.Fprintf(os.Stdout, "No nodes matching '%s'.\n", keyword)
                return nil
        }

        fmt.Fprintf(os.Stdout, "%-35s %-12s %s\n", "URI", "CATEGORY", "VERSION")
        for _, n := range wrapper.Nodes {
                fmt.Fprintf(os.Stdout, "%-35s %-12s %s\n", n["uri"], n["category"], n["version"])
        }
        return nil
}

// node push
var nodePushCmd = &cobra.Command{
        Use:   "push <file>",
        Short: "Publish a node to the registry",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodePush,
}

func runNodePush(cmd *cobra.Command, args []string) error {
        filePath := args[0]

        // Read the file
        content, err := os.ReadFile(filePath)
        if err != nil {
                return fmt.Errorf("failed to read file %s: %w", filePath, err)
        }

        // Build manifest
        manifest := map[string]interface{}{
                "uri":         nodePushURI,
                "category":    nodePushCategory,
                "version":     nodePushVersion,
                "description": fmt.Sprintf("Node %s", nodePushURI),
        }
        manifestJson, _ := json.Marshal(manifest)

        // Build push request
        pushReq := map[string]interface{}{
                "uri":         nodePushURI,
                "category":    nodePushCategory,
                "version":     nodePushVersion,
                "manifest":    string(manifestJson),
                "content":     content,
                "contentType": nodePushType,
        }

        // Send to catalog via the control plane
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()

        resp, err := c.RawPost(ctx, "/api/v1/registry/nodes", pushReq)
        if err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to push node: %w", err))
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
                body, _ := io.ReadAll(resp.Body)
                return newPrinter().PrintError(fmt.Errorf("push failed (HTTP %d): %s", resp.StatusCode, string(body)))
        }

        fmt.Fprintf(os.Stdout, "✓ Node %s pushed to registry (%d bytes, type=%s)\n",
                nodePushURI, len(content), nodePushType)
        return nil
}

// node pull
var nodePullCmd = &cobra.Command{
        Use:   "pull <uri>",
        Short: "Download a node from the registry",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodePull,
}

func runNodePull(cmd *cobra.Command, args []string) error {
        uri := args[0]

        c := newClient()
        ctx, cancel := ctx()
        defer cancel()

        resp, err := c.RawPost(ctx, "/api/v1/registry/nodes/pull", map[string]string{"uri": uri})
        if err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to pull node: %w", err))
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
                body, _ := io.ReadAll(resp.Body)
                return newPrinter().PrintError(fmt.Errorf("pull failed (HTTP %d): %s", resp.StatusCode, string(body)))
        }

        var pkg map[string]interface{}
        if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to decode response: %w", err))
        }

        // Save content to file
        content, _ := pkg["content"].(string) // base64 encoded
        ct, _ := pkg["contentType"].(string)

        ext := ".ts"
        if ct == "shared-library" {
                ext = ".so"
        }
        // Sanitize URI for filename
        filename := strings.ReplaceAll(uri, "/", "-") + ext
        outPath := filepath.Join(".", filename)

        if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to write file: %w", err))
        }

        fmt.Fprintf(os.Stdout, "✓ Node %s pulled to %s (%d bytes, type=%s)\n",
                uri, outPath, len(content), ct)
        return nil
}
