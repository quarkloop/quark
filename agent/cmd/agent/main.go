// agent is the quark agent binary.
//
// It runs a long-lived agent process that serves HTTP endpoints for health,
// stats, and chat, and executes the autonomous ORIENT→PLAN→DISPATCH→MONITOR→ASSESS
// loop for a single space.
package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/agent/pkg/infra/signals"
	"github.com/quarkloop/agent/pkg/runtime"
	"github.com/quarkloop/core/pkg/toolkit"
)

func main() {
	root := toolkit.NewToolCommand("agent", "agent runtime process")

	root.AddCommand(runCmd())

	toolkit.Execute(root)
}

func runCmd() *cobra.Command {
	var (
		agentID   string
		dir       string
		port      int
		apiServer string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the agent runtime for a space",
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentID == "" {
				return fmt.Errorf("--id is required")
			}
			if dir == "" {
				return fmt.Errorf("--dir is required")
			}

			ctx, cancel := signals.NotifyContext(context.Background())
			defer cancel()

			cfg := &runtime.Config{
				AgentID:   agentID,
				Dir:       dir,
				Port:      port,
				APIServer: apiServer,
			}

			r, err := runtime.New(cfg)
			if err != nil {
				return fmt.Errorf("runtime init: %w", err)
			}
			defer r.Close()

			return r.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&agentID, "id", "", "Agent ID assigned by the api-server (required)")
	cmd.Flags().StringVar(&dir, "dir", ".", "Space project directory containing the Quarkfile")
	cmd.Flags().IntVar(&port, "port", 7100, "Port for this agent's HTTP API")
	cmd.Flags().StringVar(&apiServer, "api-server", "", "optional api-server base URL for agent health reports")

	return cmd
}
