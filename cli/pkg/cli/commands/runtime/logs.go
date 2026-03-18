package runtime

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
)

// LogsCLI returns the "logs" command.
func LogsCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <id>",
		Short: "Stream logs from a running space",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return api.NewClientApi(apiServerURL()).StreamLogs(cmd.Context(), args[0], func(chunk string) {
				fmt.Print(chunk)
			})
		},
	}
}
