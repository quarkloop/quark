package runtime

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
)

// EventsCLI returns the "events" command.
func EventsCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "events <id>",
		Short: "Stream lifecycle events from a running space",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return api.NewClientApi(apiServerURL()).StreamEvents(cmd.Context(), args[0], func(chunk string) {
				fmt.Print(chunk)
			})
		},
	}
}
