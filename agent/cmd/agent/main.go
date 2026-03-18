// agent is the quark agent binary.
//
// It runs in one of two modes selected by the --mode flag:
//
//	supervisor  Long-lived process, HTTP server, owns the agent loop for one
//	            space. Spawned by the api-server; one per space.
//
//	worker      Short-lived process, executes a single plan step and exits.
//	            Spawned by a supervisor; communicates results via IPC socket.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/quarkloop/agent/pkg/infra/signals"
	"github.com/quarkloop/agent/pkg/supervisor"
	"github.com/quarkloop/agent/pkg/worker"
)

func main() {
	root := &cobra.Command{
		Use:          "agent",
		Short:        "quark agent — supervisor and worker process",
		SilenceUsage: true,
	}

	root.AddCommand(supervisorCmd())
	root.AddCommand(workerCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func supervisorCmd() *cobra.Command {
	var (
		spaceID   string
		dir       string
		port      int
		apiServer string
		ipcSocket string
	)

	cmd := &cobra.Command{
		Use:   "supervisor",
		Short: "Start a supervisor agent for a space (long-lived, HTTP server)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if spaceID == "" {
				return fmt.Errorf("--id is required")
			}
			if dir == "" {
				return fmt.Errorf("--dir is required")
			}

			ctx, cancel := signals.NotifyContext(context.Background())
			defer cancel()

			cfg := &supervisor.Config{
				SpaceID:   spaceID,
				Dir:       dir,
				Port:      port,
				APIServer: apiServer,
				IPCSocket: ipcSocket,
			}

			s, err := supervisor.New(cfg)
			if err != nil {
				return fmt.Errorf("supervisor init: %w", err)
			}
			defer s.Close()

			return s.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&spaceID, "id", "", "Space ID assigned by the api-server (required)")
	cmd.Flags().StringVar(&dir, "dir", ".", "Space project directory containing the Quarkfile")
	cmd.Flags().IntVar(&port, "port", 7100, "Port for this supervisor's HTTP API")
	cmd.Flags().StringVar(&apiServer, "api-server", "http://127.0.0.1:7070", "api-server base URL for health reports")
	cmd.Flags().StringVar(&ipcSocket, "ipc-socket", "", "Unix socket path for worker IPC (default: auto)")

	return cmd
}

func workerCmd() *cobra.Command {
	var (
		spaceID   string
		stepID    string
		ipcSocket string
	)

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Execute a single plan step and report the result (short-lived)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if spaceID == "" {
				return fmt.Errorf("--space-id is required")
			}
			if stepID == "" {
				return fmt.Errorf("--step-id is required")
			}
			if ipcSocket == "" {
				return fmt.Errorf("--ipc-socket is required")
			}

			ctx, cancel := signals.NotifyContext(context.Background())
			defer cancel()

			cfg := &worker.Config{
				SpaceID:   spaceID,
				StepID:    stepID,
				IPCSocket: ipcSocket,
			}

			w, err := worker.New(cfg)
			if err != nil {
				return fmt.Errorf("worker init: %w", err)
			}

			return w.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&spaceID, "space-id", "", "Space ID this worker belongs to (required)")
	cmd.Flags().StringVar(&stepID, "step-id", "", "Plan step ID to execute (required)")
	cmd.Flags().StringVar(&ipcSocket, "ipc-socket", "", "Supervisor IPC socket path (required)")

	return cmd
}
