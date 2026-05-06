// Package runtime handles launching and stopping runtime processes.
package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/quarkloop/supervisor/pkg/api"
)

// StopCallback is invoked under the registry lock when a runtime process exits.
type StopCallback func(runtimeID string)

// Launcher manages runtime process lifecycle.
type Launcher struct {
	runtimeBin    string
	supervisorURL string
	onStop        StopCallback
}

// NewLauncher creates a Launcher that spawns the given runtime binary. supervisorURL is
// forwarded to the child as QUARK_SUPERVISOR_URL so the runtime can subscribe to
// supervisor SSE events and issue KB/config reads.
// onStop is called under the registry lock when a runtime process exits.
func NewLauncher(runtimeBin, supervisorURL string, onStop StopCallback) *Launcher {
	return &Launcher{runtimeBin: runtimeBin, supervisorURL: supervisorURL, onStop: onStop}
}

// Start launches a runtime process for the registry entry. On success it
// sets entry.Cmd, entry.PID, entry.Status = RuntimeRunning. When the
// process exits the status is transitioned to RuntimeStopped.
func (l *Launcher) Start(ctx context.Context, rt *Runtime, quarkfileEnv []string) error {
	if rt.Port() == 0 {
		return fmt.Errorf("launch runtime %s: port not assigned", rt.ID())
	}
	// Use a detached context: the child runtime's lifetime is owned by the
	// registry, not by the HTTP request that spawned it.
	// ctx is intentionally unused; the goroutine manages its own lifecycle.
	cmd := exec.Command(l.runtimeBin,
		"start",
		"--port", fmt.Sprintf("%d", rt.Port()),
	)
	cmd.Dir = rt.WorkingDir()
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("QUARK_RUNTIME_ID=%s", rt.ID()),
		fmt.Sprintf("QUARK_SPACE=%s", rt.Space()),
	)
	cmd.Env = append(cmd.Env, quarkfileEnv...)
	if l.supervisorURL != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("QUARK_SUPERVISOR_URL=%s", l.supervisorURL))
	}
	if rt.PluginsDir() != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("QUARK_PLUGINS_DIR=%s", rt.PluginsDir()))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch runtime %s: %w", rt.ID(), err)
	}

	rt.SetCmd(cmd)
	rt.SetPID(cmd.Process.Pid)
	rt.SetStartedAt(time.Now().UTC())
	rt.SetStatus(api.RuntimeRunning)

	go func() {
		if err := cmd.Wait(); err != nil {
			slog.Error("runtime exited with error", "runtime_id", rt.ID(), "error", err)
		}
		if l.onStop != nil {
			l.onStop(rt.ID())
		}
	}()

	return nil
}

// Stop sends SIGTERM to the runtime process. The caller must hold the registry
// write lock for the duration of this call to avoid a data race on rt.Status.
func (l *Launcher) Stop(rt *Runtime) error {
	if rt.Cmd() == nil || rt.Cmd().Process == nil {
		return fmt.Errorf("runtime %s is not running", rt.ID())
	}
	rt.SetStatus(api.RuntimeStopping)
	if err := rt.Cmd().Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal runtime %s: %w", rt.ID(), err)
	}
	return nil
}
