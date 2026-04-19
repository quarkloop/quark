// Package runtime handles launching and stopping agent processes.
package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/registry"
)

// Launcher manages agent process lifecycle.
type Launcher struct {
	agentBin      string
	supervisorURL string
}

// New creates a Launcher that spawns the given agent binary. supervisorURL is
// forwarded to the child as QUARK_SUPERVISOR_URL so the agent can subscribe to
// supervisor SSE events and issue KB/config reads.
func New(agentBin, supervisorURL string) *Launcher {
	return &Launcher{agentBin: agentBin, supervisorURL: supervisorURL}
}

// Start launches an agent process for the registry entry. On success it
// sets entry.Cmd, entry.PID, entry.Status = AgentRunning. When the
// process exits the status is transitioned to AgentStopped.
func (l *Launcher) Start(ctx context.Context, agent *registry.Agent) error {
	if agent.Port == 0 {
		return fmt.Errorf("launch agent %s: port not assigned", agent.ID)
	}
	// Use a detached context: the child agent's lifetime is owned by the
	// registry, not by the HTTP request that spawned it.
	_ = ctx
	cmd := exec.Command(l.agentBin,
		"start",
		"--port", fmt.Sprintf("%d", agent.Port),
	)
	cmd.Dir = agent.WorkingDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("QUARK_AGENT_ID=%s", agent.ID),
		fmt.Sprintf("QUARK_SPACE=%s", agent.Space),
	)
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
		_ = cmd.Wait()
		agent.Status = api.AgentStopped
		agent.PID = 0
		agent.Cmd = nil
	}()

	return nil
}

// Stop sends SIGTERM to the agent process.
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
