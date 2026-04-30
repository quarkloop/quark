package runtimecmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/util"
	spacemodel "github.com/quarkloop/pkg/space"
	"github.com/quarkloop/supervisor/pkg/api"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

var flags struct {
	port    int
	timeout time.Duration
}

// RunCLI returns the "run" command.
func RunCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [dir]",
		Short: "Start the agent for the current space via the supervisor",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runRun,
	}

	cmd.Flags().IntVar(&flags.port, "port", 0, "Port for the agent HTTP API (0 = supervisor picks)")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "Maximum time to wait for the agent to become ready")
	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	workingDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	if _, err := os.Stat(workingDir); err != nil {
		return fmt.Errorf("working dir: %w", err)
	}

	data, err := spacemodel.ReadQuarkfile(workingDir)
	if err != nil {
		return err
	}

	space, err := spacemodel.NameFromQuarkfile(data)
	if err != nil {
		return err
	}

	sup := supclient.New()
	if _, err := sup.GetSpace(cmd.Context(), space); err != nil {
		if !supclient.IsNotFound(err) {
			return fmt.Errorf("lookup space: %w", err)
		}
		// Space doesn't exist yet.
		return fmt.Errorf("space %s not found", space)
	}

	agent, err := sup.StartAgent(cmd.Context(), space, flags.port)
	if err != nil {
		return fmt.Errorf("start agent: %w", err)
	}

	if err := waitUntilRunning(cmd.Context(), sup, agent.ID, flags.timeout); err != nil {
		return fmt.Errorf("agent failed to become ready: %w", err)
	}

	util.Successf("Agent started (space=%s, pid=%d, port=%d)", agent.Space, agent.PID, agent.Port)
	fmt.Printf("  Agent ID: %s\n", agent.ID)
	fmt.Printf("  URL:      %s\n", agent.URL())
	fmt.Println("\nUse 'quark activity --follow' to stream activity.")
	fmt.Println("Use 'quark stop' to stop.")
	return nil
}

// waitUntilRunning polls the supervisor until the agent reports running.
func waitUntilRunning(ctx context.Context, sup *supclient.Client, agentID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		agent, err := sup.GetAgent(ctx, agentID)
		if err == nil && agent.Status == api.AgentRunning {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s", timeout)
}
