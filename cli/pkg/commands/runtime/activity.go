package runtime

import (
	"fmt"

	"github.com/spf13/cobra"

	agentclient "github.com/quarkloop/agent-client"
)

// ActivityCLI returns the "activity" command.
func ActivityCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activity <agent-url>",
		Short: "Stream agent activity",
		Long: `Connect to a running agent and stream its activity events.

Example:
  quark activity http://127.0.0.1:7100`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := agentclient.New(args[0])
			return streamAgentActivity(cmd.Context(), client, func(chunk string) {
				fmt.Println(chunk)
			})
		},
	}
	return cmd
}
