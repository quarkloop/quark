package runtime

import (
	"github.com/spf13/cobra"

	"github.com/quarkloop/agent/pkg/infra/term"
	"github.com/quarkloop/api-server/pkg/api"
)

// InspectCLI returns the "inspect" command.
func InspectCLI() *cobra.Command {
	var agentURL string

	cmd := &cobra.Command{
		Use:   "inspect [id]",
		Short: "Show full details of an agent by ID or direct agent URL",
		Args: func(cmd *cobra.Command, args []string) error {
			if agentURL != "" {
				return cobra.MaximumNArgs(0)(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentURL != "" {
				return inspectDirectAgent(cmd.Context(), agentURL)
			}

			client := api.NewClientApi(apiServerURL())
			s, err := api.NewClientApi(apiServerURL()).GetSpace(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			mode := ""
			if s.Port > 0 && s.Status == "running" {
				if resp, err := client.Agent(s.ID).Mode(cmd.Context()); err == nil {
					mode = resp.Mode
				}
			}
			term.PrintSpaceDetail(term.SpaceRow{
				ID: s.ID, Name: s.Name, Status: string(s.Status),
				Mode: mode, Port: s.Port, Dir: s.Dir, PID: s.PID,
				CreatedAt: s.CreatedAt,
			})
			return nil
		},
	}
	cmd.Flags().StringVar(&agentURL, "agent-url", "", "Direct agent base URL")
	return cmd
}
