// Package dataplane — single-process lifecycle.
//
// DataPlaneProcess wraps an *exec.Cmd for one spawned data-plane
// process. It exposes IsAlive, PID, Stop, and waitForReady (HTTP
// polling of /q/health/live).
//
// Same lifecycle as the Java DataPlaneProcess:
//   - start: spawn via exec.Cmd, redirect stdout+stderr to a log file
//   - waitForReady: poll http://127.0.0.1:<port>/q/health/live
//     until it returns 200 or the timeout elapses
//   - stop: SIGTERM, wait 5s, then SIGKILL if still alive
package dataplane

import (
        "fmt"
        "net/http"
        "os"
        "os/exec"
        "sync"
        "syscall"
        "time"

        "go.uber.org/zap"
)

// DataPlaneProcess is one running data-plane process.
//
// All methods are safe for concurrent use — the mutex guards the
// underlying *exec.Cmd and started flag.
type DataPlaneProcess struct {
        runtimeId string
        cmd       *exec.Cmd
        logFile   *os.File
        httpPort  int
        log       *zap.Logger

        mu      sync.Mutex
        stopped bool
}

// IsAlive reports whether the process is still running.
func (p *DataPlaneProcess) IsAlive() bool {
        p.mu.Lock()
        defer p.mu.Unlock()
        if p.cmd == nil || p.cmd.Process == nil {
                return false
        }
        // ProcessState is set when the process has exited; nil while it's
        // still running.
        return p.cmd.ProcessState == nil
}

// PID returns the OS-level process ID, or -1 if not running.
func (p *DataPlaneProcess) PID() int {
        if p.cmd == nil || p.cmd.Process == nil {
                return -1
        }
        return p.cmd.Process.Pid
}

// HTTPPort returns the HTTP port the data-plane process listens on.
func (p *DataPlaneProcess) HTTPPort() int {
        return p.httpPort
}

// RuntimeID returns the runtimeId ("shared" or "ns-<namespace>").
func (p *DataPlaneProcess) RuntimeID() string {
        return p.runtimeId
}

// waitForReady polls the data-plane's /q/health/live endpoint until
// it returns HTTP 200 or the timeout elapses. Same path as the Java
// ProcessManager uses (SmallRye Health default).
//
// Returns true if the process became ready, false on timeout or if
// the process died during the wait.
func (p *DataPlaneProcess) waitForReady(timeout time.Duration) bool {
        url := fmt.Sprintf("http://127.0.0.1:%d/q/health/live", p.httpPort)
        client := &http.Client{Timeout: 2 * time.Second}
        deadline := time.Now().Add(timeout)

        for time.Now().Before(deadline) {
                if !p.IsAlive() {
                        return false
                }
                resp, err := client.Get(url)
                if err == nil {
                        resp.Body.Close()
                        if resp.StatusCode == 200 {
                                return true
                        }
                }
                time.Sleep(500 * time.Millisecond)
        }
        return false
}

// Stop gracefully stops the data-plane process: sends SIGTERM, waits
// up to 5 seconds, then SIGKILLs if still alive.
//
// Idempotent: calling Stop on an already-stopped process is a no-op.
func (p *DataPlaneProcess) Stop() {
        p.mu.Lock()
        defer p.mu.Unlock()

        if p.stopped || p.cmd == nil || p.cmd.Process == nil {
                return
        }
        p.stopped = true

        p.log.Info("stopping data-plane process",
                zap.String("runtimeId", p.runtimeId),
                zap.Int("pid", p.cmd.Process.Pid))

        // Send SIGTERM. On Windows this is a hard kill, but we don't
        // support Windows for data-plane processes anyway.
        _ = p.cmd.Process.Signal(syscall.SIGTERM)

        // Wait up to 5s for graceful exit.
        done := make(chan error, 1)
        go func() { done <- p.cmd.Wait() }()
        select {
        case <-done:
                // exited cleanly
        case <-time.After(5 * time.Second):
                p.log.Warn("data-plane did not exit in 5s — force killing",
                        zap.String("runtimeId", p.runtimeId))
                _ = p.cmd.Process.Kill()
                <-done
        }

        if p.logFile != nil {
                _ = p.logFile.Close()
        }
        p.log.Info("data-plane process stopped", zap.String("runtimeId", p.runtimeId))
}
