package cmd

import (
        "archive/zip"
        "bytes"
        "encoding/json"
        "fmt"
        "io"
        "net/http"
        "net/url"
        "os"
        "os/exec"
        "path/filepath"
        "strings"

        "github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
        Use:   "node",
        Short: "Manage node packages in the registry",
        Long: `Manage node packages in the Quark Catalog registry.

Similar to 'docker image', these commands let you publish, pull,
list, and search for node implementations.

Nodes live in the nodes/ directory at the project root. Each node is
a directory whose path matches its URI (e.g. nodes/quark/time/schedule/timer/v1/
for URI quark/time/schedule/timer:v1). Every node directory contains:
  - manifest.json   — node identity + metadata
  - src/            — source code (node.java or node.ts)
  - build.toml      — build + package configuration
  - README.md       — developer documentation

Build + push flow:
  quarkctl node build quark/time/schedule/timer:v1
  quarkctl node push  quark/time/schedule/timer:v1

The build step compiles Java src/ to a .jar (TypeScript nodes need no
build — src/ is used as-is). The push step packages manifest.json +
the build output (or the .ts source) into a zip and sends it to the
Catalog via the control plane's /api/v1/registry/nodes endpoint.

Examples:
  quarkctl node list
  quarkctl node info quark/time/schedule/timer:v1
  quarkctl node search "cpu"
  quarkctl node build quark/time/schedule/timer:v1
  quarkctl node push  quark/time/schedule/timer:v1
  quarkctl node pull  quark/time/schedule/timer:v1`,
}

var nodePushLegacyURI    string
var nodePushLegacyVersion string
var nodePushLegacyType   string

// nodesRoot is the directory the CLI searches for node packages.
// Defaults to "./nodes" relative to the current working directory.
// Override with the QUARK_NODES_ROOT env var.
func nodesRoot() string {
        if r := os.Getenv("QUARK_NODES_ROOT"); r != "" {
                return r
        }
        return filepath.Join(".", "nodes")
}

func init() {
        rootCmd.AddCommand(nodeCmd)
        nodeCmd.AddCommand(nodeListCmd)
        nodeCmd.AddCommand(nodeInfoCmd)
        nodeCmd.AddCommand(nodeSearchCmd)
        nodeCmd.AddCommand(nodeBuildCmd)
        nodeCmd.AddCommand(nodePushCmd)
        nodeCmd.AddCommand(nodePullCmd)

        // Legacy flags for the "push <file>" form (kept for backwards compat).
        // When push is called with a URI (no --uri flag), the URI is the arg
        // and these flags are ignored.
        nodePushCmd.Flags().StringVar(&nodePushLegacyURI, "uri", "", "Node URI (legacy: required for 'push <file>' form)")
        nodePushCmd.Flags().StringVar(&nodePushLegacyVersion, "version", "1.0.0", "Node version (legacy)")
        nodePushCmd.Flags().StringVar(&nodePushLegacyType, "type", "typescript", "Content type (typescript|shared-library) (legacy)")
        // Note: --uri is NOT marked required so the URI-form (push <uri>) works.
}

// nodeURItoPath converts a URI like "quark/time/schedule/timer:v1" to a
// directory path like "nodes/quark/time/schedule/timer/v1/".
func nodeURItoPath(uri string) (string, error) {
        // Strip the :version suffix, then re-join as path/version
        idx := strings.LastIndex(uri, ":")
        if idx < 0 {
                return "", fmt.Errorf("invalid node URI %q: missing :version suffix", uri)
        }
        stem := uri[:idx]
        version := uri[idx+1:]
        if stem == "" || version == "" {
                return "", fmt.Errorf("invalid node URI %q: empty stem or version", uri)
        }
        // Replace any ':' in stem (shouldn't happen but be defensive)
        stem = strings.ReplaceAll(stem, ":", "/")
        return filepath.Join(nodesRoot(), stem, version), nil
}

// readManifest reads and parses the manifest.json in a node directory.
func readManifest(nodeDir string) (map[string]interface{}, error) {
        manifestPath := filepath.Join(nodeDir, "manifest.json")
        data, err := os.ReadFile(manifestPath)
        if err != nil {
                return nil, fmt.Errorf("cannot read manifest.json in %s: %w", nodeDir, err)
        }
        var m map[string]interface{}
        if err := json.Unmarshal(data, &m); err != nil {
                return nil, fmt.Errorf("invalid manifest.json in %s: %w", nodeDir, err)
        }
        return m, nil
}

// readBuildToml reads build.toml and returns a map of section → key → value.
// This is a minimal TOML parser — we only need [package], [build.jvm],
// [build.native], [build.typescript] sections with simple key = "value" lines.
func readBuildToml(nodeDir string) (map[string]map[string]string, error) {
        tomlPath := filepath.Join(nodeDir, "build.toml")
        data, err := os.ReadFile(tomlPath)
        if err != nil {
                // build.toml is optional for TypeScript nodes (no build step)
                return map[string]map[string]string{}, nil
        }
        sections := map[string]map[string]string{}
        current := ""
        for _, line := range strings.Split(string(data), "\n") {
                line = strings.TrimSpace(line)
                if line == "" || strings.HasPrefix(line, "#") {
                        continue
                }
                if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
                        current = line[1 : len(line)-1]
                        sections[current] = map[string]string{}
                        continue
                }
                if current == "" {
                        continue
                }
                eq := strings.Index(line, "=")
                if eq < 0 {
                        continue
                }
                key := strings.TrimSpace(line[:eq])
                val := strings.TrimSpace(line[eq+1:])
                // Strip surrounding quotes
                if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
                        val = val[1 : len(val)-1]
                }
                sections[current][key] = val
        }
        return sections, nil
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

        fmt.Fprintf(os.Stdout, "%-40s %-12s %-15s %s\n", "URI", "TYPE", "VERSION", "DESCRIPTION")
        for _, n := range wrapper.Nodes {
                uri, _ := n["uri"].(string)
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
                fmt.Fprintf(os.Stdout, "%-40s %-12s %-15s %s\n", uri, ct, ver, desc)
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

        fmt.Fprintf(os.Stdout, "%-40s %-12s %s\n", "URI", "TYPE", "VERSION")
        for _, n := range wrapper.Nodes {
                fmt.Fprintf(os.Stdout, "%-40s %-12s %s\n", n["uri"], n["contentType"], n["version"])
        }
        return nil
}

// node build — compiles src/ per build.toml.
//   - Java: javac src/*.java -d target/classes, then jar cf target/<node>-v<version>.jar -C target/classes .
//   - TypeScript: no-op (src/ is used as-is by the runtime)
//   - Other: error
var nodeBuildCmd = &cobra.Command{
        Use:   "build <uri>",
        Short: "Compile a node's src/ per build.toml (Java → .jar; TypeScript → no-op)",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodeBuild,
}

func runNodeBuild(cmd *cobra.Command, args []string) error {
        uri := args[0]
        nodeDir, err := nodeURItoPath(uri)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        if _, err := os.Stat(nodeDir); err != nil {
                return newPrinter().PrintError(fmt.Errorf("node directory not found for %s: %w", uri, err))
        }

        manifest, err := readManifest(nodeDir)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        language, _ := manifest["language"].(string)
        if language == "" {
                return newPrinter().PrintError(fmt.Errorf("manifest.json for %s is missing 'language' field", uri))
        }

        buildToml, err := readBuildToml(nodeDir)
        if err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to read build.toml for %s: %w", uri, err))
        }

        // Create target/ dir
        targetDir := filepath.Join(nodeDir, "target")
        if err := os.MkdirAll(targetDir, 0o755); err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to create target/ in %s: %w", nodeDir, err))
        }

        switch language {
        case "java":
                return buildJavaNode(uri, nodeDir, targetDir, buildToml)
        case "typescript":
                // TypeScript nodes don't need a build step — the runtime evaluates src/ directly.
                fmt.Fprintf(os.Stdout, "✓ Node %s is TypeScript — no build step (src/ used as-is by runtime)\n", uri)
                return nil
        default:
                return newPrinter().PrintError(fmt.Errorf("unsupported language %q for node %s", language, uri))
        }
}

// buildJavaNode compiles src/*.java → target/<node>-v<version>.jar.
// The classpath is built from the manifest's dependencies.java list
// (resolved against the local Maven repository ~/.m2/repository).
func buildJavaNode(uri, nodeDir, targetDir string, buildToml map[string]map[string]string) error {
        manifest, _ := readManifest(nodeDir)
        uriStr, _ := manifest["uri"].(string)
        if uriStr == "" {
                uriStr = uri
        }

        // Resolve classpath from manifest dependencies
        classpath, err := resolveJavaClasspath(manifest)
        if err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to resolve classpath for %s: %w", uri, err))
        }

        // Compile src/*.java → target/classes/
        classesDir := filepath.Join(targetDir, "classes")
        if err := os.MkdirAll(classesDir, 0o755); err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to create classes dir: %w", err))
        }

        srcDir := filepath.Join(nodeDir, "src")
        javaFiles, err := filepath.Glob(filepath.Join(srcDir, "*.java"))
        if err != nil || len(javaFiles) == 0 {
                return newPrinter().PrintError(fmt.Errorf("no .java files found in %s", srcDir))
        }

        javacArgs := []string{
                "-d", classesDir,
                "-cp", classpath,
        }
        javacArgs = append(javacArgs, javaFiles...)

        fmt.Fprintf(os.Stdout, "▶ Compiling %d Java file(s) for %s...\n", len(javaFiles), uri)
        javacCmd := exec.Command("javac", javacArgs...)
        javacCmd.Stdout = os.Stdout
        javacCmd.Stderr = os.Stderr
        if err := javacCmd.Run(); err != nil {
                return newPrinter().PrintError(fmt.Errorf("javac failed for %s: %w", uri, err))
        }

        // Determine jar output path
        jarName := ""
        if pkg, ok := buildToml["build.jvm"]; ok {
                if out, ok := pkg["output"]; ok && out != "" {
                        // Output is relative to nodeDir (e.g. "target/timer-v1.0.0.jar")
                        jarName = filepath.Join(nodeDir, out)
                }
        }
        if jarName == "" {
                // Fallback: target/<node-name>-v<version>.jar
                node, _ := manifest["node"].(string)
                version, _ := manifest["version"].(string)
                jarName = filepath.Join(targetDir, fmt.Sprintf("%s-v%s.jar", node, version))
        }

        // jar cf <jarName> -C <classesDir> .
        fmt.Fprintf(os.Stdout, "▶ Packaging %s...\n", jarName)
        jarCmd := exec.Command("jar", "cf", jarName, "-C", classesDir, ".")
        jarCmd.Stdout = os.Stdout
        jarCmd.Stderr = os.Stderr
        if err := jarCmd.Run(); err != nil {
                return newPrinter().PrintError(fmt.Errorf("jar failed for %s: %w", uri, err))
        }

        // Verify the jar is non-empty
        info, err := os.Stat(jarName)
        if err != nil || info.Size() == 0 {
                return newPrinter().PrintError(fmt.Errorf("jar output is empty or missing: %s", jarName))
        }

        fmt.Fprintf(os.Stdout, "✓ Built %s → %s (%d bytes)\n", uri, jarName, info.Size())
        return nil
}

// resolveJavaClasspath builds a classpath string from the manifest's
// dependencies.java list. Each dependency is a Maven coordinate
// (group:artifact:version); we resolve it to ~/.m2/repository/<group>/<artifact>/<version>/<artifact>-<version>.jar.
func resolveJavaClasspath(manifest map[string]interface{}) (string, error) {
        deps, _ := manifest["dependencies"].(map[string]interface{})
        javaDeps, _ := deps["java"].([]interface{})
        if len(javaDeps) == 0 {
                // No declared dependencies — use the system classpath as a fallback
                // (works when running inside a project that has core/ on the classpath)
                return os.Getenv("CLASSPATH"), nil
        }

        home, err := os.UserHomeDir()
        if err != nil {
                return "", err
        }
        m2 := filepath.Join(home, ".m2", "repository")

        var paths []string
        for _, d := range javaDeps {
                coord, _ := d.(string)
                if coord == "" {
                        continue
                }
                // Parse group:artifact:version
                parts := strings.Split(coord, ":")
                if len(parts) != 3 {
                        return "", fmt.Errorf("invalid Maven coordinate %q (expected group:artifact:version)", coord)
                }
                group := strings.ReplaceAll(parts[0], ".", string(filepath.Separator))
                artifact := parts[1]
                version := parts[2]
                jarPath := filepath.Join(m2, group, artifact, version, artifact+"-"+version+".jar")
                if _, err := os.Stat(jarPath); err != nil {
                        return "", fmt.Errorf("dependency not found in local Maven repo: %s (looked at %s)", coord, jarPath)
                }
                paths = append(paths, jarPath)
        }
        return strings.Join(paths, string(filepath.ListSeparator)), nil
}

// node push — packages manifest.json + build output (or src/) into a zip
// and sends it to the Catalog.
//
// Two forms:
//   quarkctl node push <uri>          (new form: packages the whole node dir)
//   quarkctl node push <file> --uri X (legacy form: pushes a single file)
//
// The form is selected by whether --uri was set explicitly. If --uri is set
// AND the arg is a path (not a URI), the legacy form is used. Otherwise the
// new URI form is used.
var nodePushCmd = &cobra.Command{
        Use:   "push <uri> | push <file> --uri <uri>",
        Short: "Publish a node to the registry",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodePush,
}

func runNodePush(cmd *cobra.Command, args []string) error {
        arg := args[0]

        // Determine form: if --uri is set AND arg is a file path, use legacy form.
        // Otherwise treat arg as a URI and use the new form.
        legacyForm := false
        if nodePushLegacyURI != "" && nodePushLegacyURI != arg {
                // --uri was explicitly set to something other than the arg → legacy form
                legacyForm = true
        }
        if legacyForm {
                return runNodePushLegacy(arg, nodePushLegacyURI)
        }
        return runNodePushURI(arg)
}

// runNodePushURI is the new form: arg is a URI like "quark/time/schedule/timer:v1".
// It reads manifest.json + build output (or src/) from the node directory,
// packages them into a zip, and pushes the zip to the Catalog.
func runNodePushURI(uri string) error {
        nodeDir, err := nodeURItoPath(uri)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        if _, err := os.Stat(nodeDir); err != nil {
                return newPrinter().PrintError(fmt.Errorf("node directory not found for %s: %w", uri, err))
        }

        manifest, err := readManifest(nodeDir)
        if err != nil {
                return newPrinter().PrintError(err)
        }

        // Verify the manifest URI matches the requested URI
        manifestURI, _ := manifest["uri"].(string)
        if manifestURI != "" && manifestURI != uri {
                return newPrinter().PrintError(fmt.Errorf("URI mismatch: arg=%s, manifest=%s", uri, manifestURI))
        }

        language, _ := manifest["language"].(string)
        version, _ := manifest["version"].(string)
        if version == "" {
                version = "1.0.0"
        }

        // Determine contentType based on language
        var contentType string
        var zipEntries []zipEntry
        switch language {
        case "java":
                contentType = "shared-library"
                // Find the built .jar
                buildToml, _ := readBuildToml(nodeDir)
                jarPath := ""
                if pkg, ok := buildToml["build.jvm"]; ok {
                        if out, ok := pkg["output"]; ok && out != "" {
                                jarPath = filepath.Join(nodeDir, out)
                        }
                }
                if jarPath == "" {
                        node, _ := manifest["node"].(string)
                        jarPath = filepath.Join(nodeDir, "target", fmt.Sprintf("%s-v%s.jar", node, version))
                }
                if _, err := os.Stat(jarPath); err != nil {
                        return newPrinter().PrintError(fmt.Errorf("jar not found for %s — run 'quarkctl node build %s' first: %w", uri, uri, err))
                }
                jarBytes, err := os.ReadFile(jarPath)
                if err != nil {
                        return newPrinter().PrintError(fmt.Errorf("failed to read jar %s: %w", jarPath, err))
                }
                zipEntries = []zipEntry{
                        {name: "manifest.json", data: mustJSON(manifest)},
                        {name: filepath.Base(jarPath), data: jarBytes},
                }
        case "typescript":
                contentType = "typescript"
                // Package all .ts files in src/
                srcDir := filepath.Join(nodeDir, "src")
                tsFiles, err := filepath.Glob(filepath.Join(srcDir, "*.ts"))
                if err != nil || len(tsFiles) == 0 {
                        return newPrinter().PrintError(fmt.Errorf("no .ts files found in %s", srcDir))
                }
                zipEntries = []zipEntry{{name: "manifest.json", data: mustJSON(manifest)}}
                for _, ts := range tsFiles {
                        data, err := os.ReadFile(ts)
                        if err != nil {
                                return newPrinter().PrintError(fmt.Errorf("failed to read %s: %w", ts, err))
                        }
                        zipEntries = append(zipEntries, zipEntry{name: filepath.Base(ts), data: data})
                }
        default:
                return newPrinter().PrintError(fmt.Errorf("unsupported language %q for node %s", language, uri))
        }

        // Build the zip
        zipBytes, err := buildZip(zipEntries)
        if err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to package node %s: %w", uri, err))
        }

        // Build push request — content is the zip blob
        manifestJson, _ := json.Marshal(manifest)
        pushReq := map[string]interface{}{
                "uri":         uri,
                "version":     version,
                "manifest":    string(manifestJson),
                "content":     zipBytes,
                "contentType": contentType,
        }

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

        fmt.Fprintf(os.Stdout, "✓ Node %s pushed to catalog (%d bytes, type=%s)\n",
                uri, len(zipBytes), contentType)
        return nil
}

// runNodePushLegacy is the old form: arg is a file path, --uri is the URI.
// Kept for backwards compatibility (e.g. pushing a single .ts file outside
// the nodes/ directory).
func runNodePushLegacy(filePath, uri string) error {
        content, err := os.ReadFile(filePath)
        if err != nil {
                return fmt.Errorf("failed to read file %s: %w", filePath, err)
        }

        manifest := map[string]interface{}{
                "uri":         uri,
                "version":     nodePushLegacyVersion,
                "description": fmt.Sprintf("Node %s", uri),
        }
        manifestJson, _ := json.Marshal(manifest)

        pushReq := map[string]interface{}{
                "uri":         uri,
                "version":     nodePushLegacyVersion,
                "manifest":    string(manifestJson),
                "content":     content,
                "contentType": nodePushLegacyType,
        }

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
                uri, len(content), nodePushLegacyType)
        return nil
}

// zipEntry is a single file in the zip package.
type zipEntry struct {
        name string
        data []byte
}

func buildZip(entries []zipEntry) ([]byte, error) {
        var buf bytes.Buffer
        w := zip.NewWriter(&buf)
        for _, e := range entries {
                f, err := w.Create(e.name)
                if err != nil {
                        return nil, err
                }
                if _, err := f.Write(e.data); err != nil {
                        return nil, err
                }
        }
        if err := w.Close(); err != nil {
                return nil, err
        }
        return buf.Bytes(), nil
}

func mustJSON(v interface{}) []byte {
        b, _ := json.MarshalIndent(v, "", "  ")
        return b
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

        content, _ := pkg["content"].(string) // base64 encoded
        ct, _ := pkg["contentType"].(string)

        ext := ".ts"
        if ct == "shared-library" {
                ext = ".jar"
        }
        filename := strings.ReplaceAll(uri, "/", "-") + ext
        outPath := filepath.Join(".", filename)

        if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
                return newPrinter().PrintError(fmt.Errorf("failed to write file: %w", err))
        }

        fmt.Fprintf(os.Stdout, "✓ Node %s pulled to %s (%d bytes, type=%s)\n",
                uri, outPath, len(content), ct)
        return nil
}
