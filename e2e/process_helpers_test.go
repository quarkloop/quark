//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"
)

type builtBinaries struct {
	agent string
	bash  string
	read  string
	write string
}

type startedProcess struct {
	name string
	cmd  *exec.Cmd
	logs bytes.Buffer
	done chan error
}

func quarkRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}
	return filepath.Join(filepath.Dir(file), "..")
}

func buildRequiredBinaries(t *testing.T) builtBinaries {
	t.Helper()

	root := quarkRoot(t)
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}

	return builtBinaries{
		agent: buildBinary(t, root, "./agent/cmd/agent", filepath.Join(binDir, "agent")),
		bash:  buildBinary(t, root, "./plugins/tool-bash/cmd/bash", filepath.Join(binDir, "bash")),
		read:  buildBinary(t, root, "./plugins/tool-read/cmd/read", filepath.Join(binDir, "read")),
		write: buildBinary(t, root, "./plugins/tool-write/cmd/write", filepath.Join(binDir, "write")),
	}
}

func buildBinary(t *testing.T, root, pkg, out string) string {
	t.Helper()

	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, string(output))
	}
	return out
}

func startProcess(t *testing.T, name, binary string, args []string, env []string) *startedProcess {
	t.Helper()

	cmd := exec.Command(binary, args...)
	cmd.Env = env

	proc := &startedProcess{
		name: name,
		cmd:  cmd,
		done: make(chan error, 1),
	}
	cmd.Stdout = &proc.logs
	cmd.Stderr = &proc.logs

	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", name, err)
	}
	go func() {
		proc.done <- cmd.Wait()
	}()

	t.Cleanup(func() {
		select {
		case <-proc.done:
		default:
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-proc.done
		}
		if t.Failed() {
			t.Logf("%s logs:\n%s", proc.name, proc.logs.String())
		}
	})

	return proc
}

func reservePort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForPort(t *testing.T, port int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for port %d", port)
}

func waitForAgentReady(t *testing.T, client *agentclient.Client, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := client.Health(ctx)
		cancel()
		if err == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timed out waiting for agent health endpoint")
}

func waitForCompletedPlanAfter(t *testing.T, client *agentclient.Client, after time.Time, timeout time.Duration) *agentapi.Plan {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		currentPlan, err := client.Plan(ctx)
		cancel()
		if err == nil {
			if currentPlan.UpdatedAt.After(after) || currentPlan.UpdatedAt.Equal(after) {
				for _, step := range currentPlan.Steps {
					if step.Status == agentapi.StepFailed {
						t.Fatalf("plan step failed")
					}
				}
				if currentPlan.Complete {
					return currentPlan
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("timed out waiting for plan completion")
	return nil
}

func processEnv(overrides map[string]string) []string {
	env := os.Environ()
	for key, value := range overrides {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func createFileToolsProjectV2(t *testing.T, bashURL, readURL, writeURL string) string {
	t.Helper()

	projectDir := t.TempDir()

	cfg, ok := cfgForTest(t, "OPENROUTER_API_KEY")
	if !ok {
		t.Skip("no provider configured")
	}
	qfData := fmt.Sprintf(`quark: "1.0"
meta:
  name: file-tools-e2e
model:
  provider: %s
  name: %s
plugins:
  - ref: github.com/quarkloop/tool-bash@builtin
  - ref: github.com/quarkloop/tool-read@builtin
  - ref: github.com/quarkloop/tool-write@builtin
`, cfg.provider, cfg.model)

	if err := os.WriteFile(filepath.Join(projectDir, "Quarkfile"), []byte(qfData), 0644); err != nil {
		t.Fatalf("write Quarkfile: %v", err)
	}

	_ = bashURL
	_ = readURL
	_ = writeURL

	pluginsDir := filepath.Join(projectDir, ".quark", "plugins")
	for _, name := range []string{"tool-bash", "tool-read", "tool-write"} {
		dir := filepath.Join(pluginsDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create plugin dir %s: %v", name, err)
		}
	}

	return projectDir
}

// scaffoldEmptySpace creates a bare space directory with just Quarkfile and .quark/ structure.
func scaffoldEmptySpace(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	qf := fmt.Sprintf(`quark: "1.0"
meta:
  name: %s
model:
  provider: openrouter
  name: qwen/qwen3.6-plus:free
plugins:
  - ref: quark/tool-bash
  - ref: quark/tool-read
  - ref: quark/tool-write
  - ref: quark/tool-web-search
`, name)
	if err := os.WriteFile(filepath.Join(dir, "Quarkfile"), []byte(qf), 0644); err != nil {
		t.Fatalf("write Quarkfile: %v", err)
	}
	for _, sub := range []string{"sessions", "config", "activity", "plans", "kb", "plugins"} {
		if err := os.MkdirAll(filepath.Join(dir, ".quark", sub), 0755); err != nil {
			t.Fatalf("create .quark/%s: %v", sub, err)
		}
	}
	return dir
}

// scaffoldToolSpace creates a space with actual tool plugin binaries installed.
func scaffoldToolSpace(t *testing.T, name string, binaries builtBinaries) string {
	t.Helper()
	dir := scaffoldEmptySpace(t, name)

	type toolInfo struct {
		dir     string
		binPath string
	}
	tools := []toolInfo{
		{"tool-bash", binaries.bash},
		{"tool-read", binaries.read},
		{"tool-write", binaries.write},
	}
	for _, tool := range tools {
		pluginDir := filepath.Join(dir, ".quark", "plugins", tool.dir)
		if err := os.MkdirAll(filepath.Join(pluginDir, "bin"), 0755); err != nil {
			t.Fatalf("create plugin dir: %v", err)
		}
		data, err := os.ReadFile(tool.binPath)
		if err != nil {
			t.Fatalf("read binary: %v", err)
		}
		binName := strings.TrimPrefix(tool.dir, "tool-")
		destBin := filepath.Join(pluginDir, "bin", binName)
		if err := os.WriteFile(destBin, data, 0755); err != nil {
			t.Fatalf("copy binary: %v", err)
		}
		manifest := fmt.Sprintf(`name: %s
version: "1.0.0"
type: tool
description: "%s"
author: quarkloop
license: Apache-2.0
repository: github.com/quarkloop/%s
interface:
  mode: [cli, http]
  endpoint: "/run"
`, tool.dir, tool.dir, tool.dir)
		if err := os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), []byte(manifest), 0644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}
	return dir
}

// startAndVerifyToolBinary starts a tool binary and waits for it to be ready.
func startAndVerifyToolBinary(t *testing.T, binary string, port int) int {
	t.Helper()
	startProcess(t, "tool-"+binary+"-"+fmt.Sprint(port), binary, []string{
		"serve", "--addr", fmt.Sprintf("127.0.0.1:%d", port),
	}, processEnv(nil))
	waitForPort(t, port, 10*time.Second)
	return port
}

// startAgentWithTools starts a full agent with tool binaries and returns an HTTP client.
func startAgentWithTools(t *testing.T) (*agentclient.Client, func()) {
	t.Helper()

	t.Logf("=== starting agent with tools ===")

	binaries := buildRequiredBinaries(t)
	t.Logf("binaries built: agent=%s bash=%s read=%s write=%s",
		binaries.agent, binaries.bash, binaries.read, binaries.write)

	spaceDir := scaffoldToolSpace(t, "e2e-agent", binaries)
	t.Logf("space dir: %s", spaceDir)

	agentPort := reservePort(t)
	bashPort := reservePort(t)
	readPort := reservePort(t)
	writePort := reservePort(t)

	t.Logf("ports: agent=%d bash=%d read=%d write=%d", agentPort, bashPort, readPort, writePort)

	startProcess(t, "bash", binaries.bash, []string{"serve", "--addr", fmt.Sprintf("127.0.0.1:%d", bashPort)}, processEnv(nil))
	startProcess(t, "read", binaries.read, []string{"serve", "--addr", fmt.Sprintf("127.0.0.1:%d", readPort)}, processEnv(nil))
	startProcess(t, "write", binaries.write, []string{"serve", "--addr", fmt.Sprintf("127.0.0.1:%d", writePort)}, processEnv(nil))
	waitForPort(t, bashPort, 10*time.Second)
	waitForPort(t, readPort, 10*time.Second)
	waitForPort(t, writePort, 10*time.Second)
	t.Logf("tool binaries started and ready")

	agentEnv := processEnv(map[string]string{
		"OPENROUTER_API_KEY": os.Getenv("OPENROUTER_API_KEY"),
		"ZHIPU_API_KEY":      os.Getenv("ZHIPU_API_KEY"),
	})
	startProcess(t, "agent", binaries.agent, []string{
		"run", "--id", "e2e-test", "--dir", spaceDir, "--port", fmt.Sprintf("%d", agentPort),
	}, agentEnv)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", agentPort)
	client := agentclient.New(baseURL)
	t.Logf("waiting for agent health...")
	waitForAgentReady(t, client, 30*time.Second)

	t.Logf("=== agent ready at %s ===", baseURL)
	return client, func() {}
}
