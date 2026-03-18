package space

import (
	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/cli/pkg/cli/config"
	"github.com/quarkloop/agent/pkg/infra/term"
)

// spaceLsCmd lists all spaces known to the api-server (running and stopped).
// This is the management-level view; for a quick live-only list use `quark ps`.
func spaceLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all spaces (running and stopped)",
		RunE: func(cmd *cobra.Command, args []string) error {
			spaces, err := api.NewClientApi(config.APIServerURL()).ListSpaces(cmd.Context())
			if err != nil {
				return err
			}
			rows := make([]term.SpaceRow, len(spaces))
			for i, s := range spaces {
				rows[i] = term.SpaceRow{
					ID: s.ID, Name: s.Name, Status: string(s.Status),
					Port: s.Port, Dir: s.Dir, PID: s.PID, CreatedAt: s.CreatedAt,
				}
			}
			term.PrintSpaceTable(rows)
			return nil
		},
	}
}
