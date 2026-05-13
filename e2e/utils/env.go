//go:build e2e

package utils

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/quarkloop/supervisor/pkg/api"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

// E2EEnv is the live supervisor+agent pair driven by an e2e test.
type E2EEnv struct {
	Root      string
	SpacesDir string
	Space     string
	SupURL    string
	Sup       *supclient.Client
	Agent     api.RuntimeInfo
	AgentURL  string
	HTTPC     *http.Client
}

// installSpacePlugins populates the space's plugins directory with the
// plugin manifests and their pre-built artifacts (tool binaries and provider
// .so files). The agent's api-mode loader detects the co-located binary
// and runs it directly; there is no runtime `go build`.
//
// Pre-built artifacts come from BuildAllOnce (tool binaries) and the
// repo-shipped provider .so (produced by `make build-providers`).
func installSpacePlugins(t *testing.T, env *E2EEnv, bins BuiltBinaries) {
	t.Helper()
	pluginsDir := filepath.Join(env.SpacesDir, env.Space, "plugins")
	srcRoot := filepath.Join(QuarkRoot(t), "plugins")

	// installTool lays out a tool plugin exactly the way production installs
	// do: manifest + the binary + (optionally) the lib-mode plugin.so. The
	// agent's pluginmanager prefers lib mode when the .so is present and
	// falls back to api mode otherwise, so shipping both proves both
	// code paths work.
	installTool := func(name, binPath, libPath string) {
		dst := filepath.Join(pluginsDir, "tools", name)
		if err := os.MkdirAll(dst, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dst, err)
		}
		copyFile(t, filepath.Join(srcRoot, "tools", name, "manifest.yaml"), filepath.Join(dst, "manifest.yaml"), 0o644)
		copyFile(t, binPath, filepath.Join(dst, name), 0o755)
		if libPath != "" {
			copyFile(t, libPath, filepath.Join(dst, "plugin.so"), 0o644)
		}
	}
	installTool("bash", bins.Bash, bins.BashLib)
	installTool("fs", bins.FS, bins.FSLib)

	providerSrc := filepath.Join(srcRoot, "providers", "openrouter")
	providerLib := bins.OpenRouterLib
	if providerLib == "" {
		providerLib = filepath.Join(providerSrc, "plugin.so")
	}
	if _, err := os.Stat(providerLib); err == nil {
		dst := filepath.Join(pluginsDir, "providers", "openrouter")
		if err := os.MkdirAll(dst, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dst, err)
		}
		copyFile(t, filepath.Join(providerSrc, "manifest.yaml"), filepath.Join(dst, "manifest.yaml"), 0o644)
		copyFile(t, providerLib, filepath.Join(dst, "plugin.so"), 0o755)
	}
}

func copyFile(t *testing.T, src, dst string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

// quarkfileFor returns the raw bytes of a minimal Quarkfile for a space.
func quarkfileFor(name, provider, model string) []byte {
	env := ""
	if provider != "noop" {
		env = `  env:
    - OPENROUTER_API_KEY
`
	}
	qf := fmt.Sprintf(`quark: "1.0"
meta:
  name: %s
  version: "0.1.0"
model:
  provider: %s
  name: %s
%s
plugins:
  - ref: quark/tool-bash
  - ref: quark/tool-fs
`, name, provider, model, env)
	return []byte(qf)
}

// startSupervisor launches a supervisor subprocess with an isolated spaces
// root and returns the client, base URL, spaces dir, and process handle. The
// handle lets tests wait for log markers from the supervisor or its agent
// child (whose stdio is inherited into the same buffer).
func startSupervisor(t *testing.T, bins BuiltBinaries) (*supclient.Client, string, string, *StartedProcess) {
	t.Helper()

	spacesDir := filepath.Join(t.TempDir(), "spaces")
	if err := os.MkdirAll(spacesDir, 0o755); err != nil {
		t.Fatalf("mkdir spaces: %v", err)
	}
	port := ReservePort(t)

	env := ProcessEnv(map[string]string{
		"QUARK_SPACES_ROOT": spacesDir,
	})
	proc := StartProcess(t, "supervisor", bins.Supervisor, []string{
		"start",
		"--port", fmt.Sprint(port),
		"--runtime", bins.Agent,
	}, env)

	supURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	// Supervisor exposes GET /v1/spaces for liveness.
	WaitForURL(t, supURL+"/v1/spaces", 10*time.Second)

	sup := supclient.New(supclient.WithBaseURL(supURL))
	return sup, supURL, spacesDir, proc
}

// StartOptions tunes the fixture StartE2E builds. Zero-valued options yield
// the default behaviour (lib mode for tools when .so is available, binary
// otherwise).
type StartOptions struct {
	// ForceBinaryTools, when true, omits the tool plugin.so files from the
	// installed space so the agent's pluginmanager must fall back to
	// api-mode loading. Used to test binary fallback end-to-end.
	ForceBinaryTools bool
	// DisableServiceDiscovery keeps legacy provider/tool E2Es focused on plugin
	// behavior instead of adding the runtime service catalog tool.
	DisableServiceDiscovery bool
}

// StartE2E boots a supervisor, registers a space, installs plugins, and
// launches an agent. Tests use the returned env to create sessions and
// interact with the agent.
func StartE2E(t *testing.T, withProvider bool, opts ...StartOptions) *E2EEnv {
	t.Helper()

	var opt StartOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	cfg, ok := CfgForTest(t, "OPENROUTER_API_KEY")
	if withProvider && !ok {
		t.Skip("no provider configured (set OPENROUTER_API_KEY)")
	}

	bins := BuildAllOnce(t)
	if opt.ForceBinaryTools {
		bins.BashLib, bins.FSLib = "", ""
	}
	if opt.DisableServiceDiscovery {
		t.Setenv("QUARK_DISABLE_SERVICE_DISCOVERY", "true")
	}
	sup, supURL, spacesDir, proc := startSupervisor(t, bins)

	spaceName := fmt.Sprintf("e2e-%d", time.Now().UnixNano())
	provider := "openrouter"
	model := "noop/noop"
	if withProvider {
		provider = cfg.Provider
		model = cfg.Model
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	workingDir := t.TempDir()
	if _, err := sup.CreateSpace(ctx, spaceName, quarkfileFor(spaceName, provider, model), workingDir); err != nil {
		t.Fatalf("create space: %v", err)
	}

	env := &E2EEnv{
		Root:      QuarkRoot(t),
		SpacesDir: spacesDir,
		Space:     spaceName,
		SupURL:    supURL,
		Sup:       sup,
		HTTPC:     &http.Client{Timeout: 30 * time.Second},
	}

	installSpacePlugins(t, env, bins)

	agentPort := ReservePort(t)
	info, err := sup.StartRuntime(ctx, spaceName, agentPort)
	if err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	env.Agent = info
	env.AgentURL = fmt.Sprintf("http://127.0.0.1:%d", info.Port)

	WaitForURL(t, env.AgentURL+"/health", 30*time.Second)
	// Wait for the agent's SSE subscription to the supervisor to go live,
	// otherwise the very first session event can be published before any
	// subscriber is attached and silently dropped.
	proc.WaitForLog(t, "supervisor event stream ready", 10*time.Second)
	t.Logf("supervisor at %s, agent at %s (space=%s)", supURL, env.AgentURL, spaceName)
	return env
}
