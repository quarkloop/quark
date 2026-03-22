package runtime

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ToolProcess represents a running tool binary managed by the orchestrator.
type ToolProcess struct {
	Name    string
	Port    int
	BinPath string
	Cmd     *exec.Cmd
}

// ToolOrchestrator manages the lifecycle of tool HTTP server processes.
// It starts tool binaries before the agent and shuts them down after.
type ToolOrchestrator struct {
	mu        sync.Mutex
	processes []*ToolProcess
	binDir    string
	spaceDir  string
}

// NewToolOrchestrator creates an orchestrator that looks for tool binaries
// in binDir and runs them with spaceDir as the working directory.
func NewToolOrchestrator(binDir, spaceDir string) *ToolOrchestrator {
	return &ToolOrchestrator{binDir: binDir, spaceDir: spaceDir}
}

// toolBinaryName maps a Quarkfile tool ref to the expected binary name.
// e.g. "quark/bash" → "bash", "quark/read" → "read", "bash" → "bash"
func toolBinaryName(ref string) string {
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '/' {
			return ref[i+1:]
		}
	}
	return ref
}

// portFromEndpoint extracts the port number from an endpoint URL.
// e.g. "http://127.0.0.1:8091/run" → 8091
func portFromEndpoint(endpoint string) (int, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return 0, fmt.Errorf("parse endpoint %q: %w", endpoint, err)
	}
	port := u.Port()
	if port == "" {
		return 0, fmt.Errorf("no port in endpoint %q", endpoint)
	}
	var p int
	if _, err := fmt.Sscanf(port, "%d", &p); err != nil {
		return 0, fmt.Errorf("invalid port %q: %w", port, err)
	}
	return p, nil
}

// Start launches a tool process on the given port. Blocks until the
// tool's HTTP endpoint responds to health probes (or times out).
func (o *ToolOrchestrator) Start(name, ref string, port int) (*ToolProcess, error) {
	binName := toolBinaryName(ref)
	binPath := filepath.Join(o.binDir, binName)

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("tool binary not found: %s", binPath)
	}

	cmd := exec.Command(binPath,
		"--port", fmt.Sprintf("%d", port),
		"--dir", o.spaceDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	if err := waitHTTP(healthURL, 10*time.Second); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("tool %s not ready on port %d: %w", name, port, err)
	}

	proc := &ToolProcess{
		Name:    name,
		Port:    port,
		BinPath: binPath,
		Cmd:     cmd,
	}

	o.mu.Lock()
	o.processes = append(o.processes, proc)
	o.mu.Unlock()

	log.Printf("orchestrator: started %s on port %d (pid %d)", name, port, cmd.Process.Pid)
	return proc, nil
}

// Shutdown gracefully stops all managed tool processes.
func (o *ToolOrchestrator) Shutdown() {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, proc := range o.processes {
		if proc.Cmd == nil || proc.Cmd.Process == nil {
			continue
		}
		log.Printf("orchestrator: stopping %s (pid %d)", proc.Name, proc.Cmd.Process.Pid)
		_ = proc.Cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func(c *exec.Cmd) { done <- c.Wait() }(proc.Cmd)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = proc.Cmd.Process.Kill()
		}
	}
	o.processes = nil
}
