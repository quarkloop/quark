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
	"testing"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"
	"github.com/quarkloop/tools/space/pkg/quarkfile"
	"github.com/quarkloop/tools/space/pkg/repo"
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
	return filepath.Join(filepath.Dir(file), "..", "..")
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
		bash:  buildBinary(t, root, "./tools/bash/cmd/bash", filepath.Join(binDir, "bash")),
		read:  buildBinary(t, root, "./tools/read/cmd/read", filepath.Join(binDir, "read")),
		write: buildBinary(t, root, "./tools/write/cmd/write", filepath.Join(binDir, "write")),
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
	address := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", address)
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
						t.Fatalf("plan step failed: %s", mustJSON(t, currentPlan))
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

func scaffoldRegistryForTestHome(t *testing.T, homeDir string) {
	t.Helper()

	t.Setenv("HOME", homeDir)
	if err := repo.ScaffoldRegistry(); err != nil {
		t.Fatalf("scaffold registry: %v", err)
	}
}

func createFileToolsProject(t *testing.T, cfg providerConfig, bashURL, readURL, writeURL string) string {
	t.Helper()

	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, "prompts"), 0755); err != nil {
		t.Fatalf("create prompts dir: %v", err)
	}

	supervisorPrompt := `You are the supervisor agent for this Quark space.

You have access to the "bash" tool, the "read" tool, and the "write" tool.

When you need to run or verify a shell command, respond with only this fenced JSON block:

` + "```tool\n" + `{"name":"bash","input":{"cmd":"<command>"}}` + "\n```" + `

When you need to inspect a file, respond with only this fenced JSON block:

` + "```tool\n" + `{"name":"read","input":{"path":"<path>"}}` + "\n```" + `

When you need to read a range, you may include start_line and end_line.

When you need to write or update a file, respond with only this fenced JSON block:

` + "```tool\n" + `{"name":"write","input":{"path":"<path>","operation":"write","content":"<text>"}}` + "\n```" + `

For precise code edits, use operation "edit" with 1-based line and column positions where the end position is exclusive.

Rules:
- Use the bash tool to verify Python output when the task requires exact runtime behavior.
- Use the read tool for inspection and the write tool for file creation or updates.
- For update tasks on an existing file, inspect it with the read tool before any write call.
- Prefer the write edit operation when changing existing code.
- Do not invent tool results or final file contents.
- Do not claim a task is complete until the required tool calls have succeeded.
- After the required tool calls are done, return a short final summary.
`
	if err := os.WriteFile(filepath.Join(projectDir, "prompts", "supervisor.txt"), []byte(supervisorPrompt), 0644); err != nil {
		t.Fatalf("write supervisor prompt: %v", err)
	}

	qf := &quarkfile.Quarkfile{
		Quark: "1.0",
		Meta: quarkfile.Meta{
			Name: "file-tools-e2e",
		},
		Model: quarkfile.Model{
			Provider: cfg.provider,
			Name:     cfg.model,
		},
		Supervisor: quarkfile.Supervisor{
			Agent:  "quark/supervisor@latest",
			Prompt: "./prompts/supervisor.txt",
		},
		Tools: []quarkfile.Tool{
			{
				Ref:  "quark/bash",
				Name: "bash",
				Config: map[string]string{
					"endpoint": bashURL,
				},
			},
			{
				Ref:  "quark/read",
				Name: "read",
				Config: map[string]string{
					"endpoint": readURL,
				},
			},
			{
				Ref:  "quark/write",
				Name: "write",
				Config: map[string]string{
					"endpoint": writeURL,
				},
			},
		},
		Restart: "never",
	}
	if err := quarkfile.Save(projectDir, qf); err != nil {
		t.Fatalf("save Quarkfile: %v", err)
	}

	now := time.Now()
	lock := &quarkfile.LockFile{
		Quark:      qf.Quark,
		ResolvedAt: &now,
		Agents: []quarkfile.LockedAgent{
			{Ref: qf.Supervisor.Agent, Resolved: qf.Supervisor.Agent},
		},
		Tools: []quarkfile.LockedTool{
			{Ref: "quark/bash", Resolved: "quark/bash"},
			{Ref: "quark/read", Resolved: "quark/read"},
			{Ref: "quark/write", Resolved: "quark/write"},
		},
	}
	if err := quarkfile.SaveLock(projectDir, lock); err != nil {
		t.Fatalf("save lock file: %v", err)
	}

	return projectDir
}

func processEnv(overrides map[string]string) []string {
	env := os.Environ()
	for key, value := range overrides {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}
