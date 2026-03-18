package runtime

import (
	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/api-server/pkg/space"
	"github.com/quarkloop/agent/pkg/infra/term"
)

// PsCLI returns the "ps" command.
func PsCLI() *cobra.Command {
	var all bool
	var statusFilter string

	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List running spaces",
		Long: `List spaces managed by the api-server.

By default only running spaces are shown. Use --all to include stopped,
failed, and created spaces. Use --status to filter by a specific status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			spaces, err := api.NewClientApi(apiServerURL()).ListSpaces(cmd.Context())
			if err != nil {
				return err
			}

			var rows []term.SpaceRow
			for _, s := range spaces {
				// Default: only show active spaces unless --all is set.
				if !all && statusFilter == "" {
					switch s.Status {
					case space.StatusRunning, space.StatusStarting, space.StatusStopping:
						// include
					default:
						continue
					}
				}
				// --status filter takes precedence over --all.
				if statusFilter != "" && string(s.Status) != statusFilter {
					continue
				}
				rows = append(rows, term.SpaceRow{
					ID: s.ID, Name: s.Name, Status: string(s.Status),
					Port: s.Port, Dir: s.Dir, PID: s.PID, CreatedAt: s.CreatedAt,
				})
			}

			term.PrintSpaceTable(rows)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Show all spaces (including stopped and failed)")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (created, starting, running, stopping, stopped, failed)")
	return cmd
}
