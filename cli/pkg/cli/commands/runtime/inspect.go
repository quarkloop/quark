package runtime

import (
	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/agent/pkg/infra/term"
)

// InspectCLI returns the "inspect" command.
func InspectCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <id>",
		Short: "Show full details of a space",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := api.NewClientApi(apiServerURL()).GetSpace(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			term.PrintSpaceDetail(term.SpaceRow{
				ID: s.ID, Name: s.Name, Status: string(s.Status),
				Port: s.Port, Dir: s.Dir, PID: s.PID, CreatedAt: s.CreatedAt,
			})
			return nil
		},
	}
}
