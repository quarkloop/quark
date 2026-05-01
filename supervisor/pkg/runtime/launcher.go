// Package runtime handles launching and stopping agent processes.
package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/registry"
)

// StopCallback is invoked under the registry lock when an agent process exits.
type StopCallback func(agentID string)

// Launcher manages agent process lifecycle.
type Launcher struct {
	agentBin      string
	supervisorURL string
	onStop        StopCallback
}

// New creates a Launcher that spawns the given agent binary. supervisorURL is
// forwarded to the child as QUARK_SUPERVISOR_URL so the agent can subscribe to
// supervisor SSE events and issue KB/config reads.
// onStop is called under the registry lock when an agent process exits.
func New(agentBin, supervisorURL string, onStop StopCallback) *Launcher {
	return &Launcher{agentBin: agentBin, supervisorURL: supervisorURL, onStop: onStop}
}

// Start launches an agent process for the registry entry. On success it
// sets entry.Cmd, entry.PID, entry.Status = AgentRunning. When the
// process exits the status is transitioned to AgentStopped.
func (l *Launcher) Start(ctx context.Context, agent *registry.Agent, quarkfileEnv []string) error {
	if agent.Port == 0 {
		return fmt.Errorf("launch agent %s: port not assigned", agent.ID)
	}
	// Use a detached context: the child agent's lifetime is owned by the
	// registry, not by the HTTP request that spawned it.
	// ctx is intentionally unused; the goroutine manages its own lifecycle.
	cmd := exec.Command(l.agentBin,
		"start",
		"--port", fmt.Sprintf("%d", agent.Port),
	)
	cmd.Dir = agent.WorkingDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("QUARK_AGENT_ID=%s", agent.ID),
		fmt.Sprintf("QUARK_SPACE=%s", agent.Space),
	)
	cmd.Env = append(cmd.Env, quarkfileEnv...)
	if l.supervisorURL != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("QUARK_SUPERVISOR_URL=%s", l.supervisorURL))
	}
	if agent.PluginsDir != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("QUARK_PLUGINS_DIR=%s", agent.PluginsDir))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch agent %s: %w", agent.ID, err)
	}

	agent.Cmd = cmd
	agent.PID = cmd.Process.Pid
	agent.Status = api.AgentRunning

	go func() {
		if err := cmd.Wait(); err != nil {
			slog.Error("agent exited with error", "agent_id", agent.ID, "error", err)
		}
		if l.onStop != nil {
			l.onStop(agent.ID)
		}
	}()

	return nil
}

// Stop sends SIGTERM to the agent process. The caller must hold the registry
// write lock for the duration of this call to avoid a data race on agent.Status.
func (l *Launcher) Stop(agent *registry.Agent) error {
	if agent.Cmd == nil || agent.Cmd.Process == nil {
		return fmt.Errorf("agent %s is not running", agent.ID)
	}
	agent.Status = api.AgentStopping
	if err := agent.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal agent %s: %w", agent.ID, err)
	}
	return nil
}
