package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/quarkloop/supervisor/pkg/server"
)

var port int
var runtimeBin string

// StartCmd creates the "supervisor start" command.
func StartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [spaces-dir]",
		Short: "Start the supervisor server",
		Long: `Start the supervisor HTTP server that manages Spaces.

Example:
  supervisor start --port 7200`,
		RunE: runStart,
	}

	cmd.Flags().IntVarP(&port, "port", "p", 7200, "HTTP listen port")
	cmd.Flags().StringVar(&runtimeBin, "runtime", "runtime", "Path to agent runtime binary")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	// When no positional arg is given, leave SpacesDir empty so
	// server.New falls back to space.DefaultRoot (which honours
	// QUARK_SPACES_ROOT or $HOME/.quarkloop/spaces).
	var spacesDir string
	if len(args) > 0 {
		spacesDir = args[0]
	}

	cfg := server.Config{
		Port:       port,
		SpacesDir:  spacesDir,
		RuntimeBin: runtimeBin,
	}
	srv, err := server.New(cfg)
	if err != nil {
		return err
	}

	return srv.Start(context.Background())
}
