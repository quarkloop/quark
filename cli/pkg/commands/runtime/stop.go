package runtime

import (
	"fmt"

	"github.com/spf13/cobra"

	agentclient "github.com/quarkloop/agent-client"
	"github.com/quarkloop/agent/pkg/infra/term"
)

// StopCLI returns the "stop" command.
func StopCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <agent-url>",
		Short: "Gracefully stop a running agent",
		Long: `Request a graceful stop from the running agent.

Example:
  quark stop http://127.0.0.1:7100`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := agentclient.New(args[0])
			if err := client.Stop(cmd.Context()); err != nil {
				return fmt.Errorf("stop request failed: %w", err)
			}
			term.Successf("Stop requested")
			return nil
		},
	}
	return cmd
}
