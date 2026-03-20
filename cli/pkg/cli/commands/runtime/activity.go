package runtime

import (
	"fmt"

	"github.com/spf13/cobra"

	agentclient "github.com/quarkloop/agent-client"
	"github.com/quarkloop/api-server/pkg/api"
)

// ActivityCLI returns the "activity" command.
func ActivityCLI() *cobra.Command {
	var agentURL string

	cmd := &cobra.Command{
		Use:   "activity [id]",
		Short: "Stream agent activity by agent ID or direct agent URL",
		Args: func(cmd *cobra.Command, args []string) error {
			if agentURL != "" {
				return cobra.MaximumNArgs(0)(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentURL != "" {
				return streamAgentActivity(cmd.Context(), agentclient.New(agentURL), func(chunk string) {
					fmt.Println(chunk)
				})
			}
			client := api.NewClientApi(apiServerURL())
			return streamAgentActivity(cmd.Context(), client.Agent(args[0]), func(chunk string) {
				fmt.Println(chunk)
			})
		},
	}
	cmd.Flags().StringVar(&agentURL, "agent-url", "", "Direct agent base URL")
	return cmd
}
