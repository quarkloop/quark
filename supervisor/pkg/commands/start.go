package commands

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/quarkloop/supervisor/pkg/server"
)

// StartCmd creates the "supervisor start" command.
func StartCmd() *cobra.Command {
	var port int
	var agentBin string

	cmd := &cobra.Command{
		Use:   "start [spaces-dir]",
		Short: "Start the supervisor server",
		Long: `Start the supervisor HTTP server that manages Spaces.

Example:
  supervisor start --port 7200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// When no positional arg is given, leave SpacesDir empty so
			// server.New falls back to space.DefaultRoot (which honours
			// QUARK_SPACES_ROOT or $HOME/.quarkloop/spaces).
			var spacesDir string
			if len(args) > 0 {
				spacesDir = args[0]
			}

			cfg := server.Config{
				Port:      port,
				SpacesDir: spacesDir,
				AgentBin:  agentBin,
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			srv, err := server.New(cfg)
			if err != nil {
				return err
			}
			return srv.Run(ctx)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 7200, "HTTP listen port")
	cmd.Flags().StringVar(&agentBin, "agent", "agent", "Path to agent binary")

	return cmd
}
