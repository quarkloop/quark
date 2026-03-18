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
		spaceID   string
		dir       string
		port      int
		apiServer string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the agent runtime for a space",
		RunE: func(cmd *cobra.Command, args []string) error {
			if spaceID == "" {
				return fmt.Errorf("--id is required")
			}
			if dir == "" {
				return fmt.Errorf("--dir is required")
			}

			ctx, cancel := signals.NotifyContext(context.Background())
			defer cancel()

			cfg := &runtime.Config{
				SpaceID:   spaceID,
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

	cmd.Flags().StringVar(&spaceID, "id", "", "Space ID assigned by the api-server (required)")
	cmd.Flags().StringVar(&dir, "dir", ".", "Space project directory containing the Quarkfile")
	cmd.Flags().IntVar(&port, "port", 7100, "Port for this agent's HTTP API")
	cmd.Flags().StringVar(&apiServer, "api-server", "http://127.0.0.1:7070", "api-server base URL for health reports")

	return cmd
}
