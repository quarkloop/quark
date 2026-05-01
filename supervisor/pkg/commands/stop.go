package commands

import (
	"github.com/spf13/cobra"

	"github.com/quarkloop/supervisor/pkg/server"
)

// StopCmd creates the "supervisor stop" command.
func StopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "stop",
		Short:         "Stop a running supervisor server",
		SilenceErrors: true,
		RunE:          runStop,
	}

	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	return server.Stop()
}
