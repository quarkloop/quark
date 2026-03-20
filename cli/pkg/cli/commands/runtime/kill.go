package runtime

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
)

// KillCLI returns the "kill" command.
func KillCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "kill <id>",
		Short: "Force stop a running agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := api.NewClientApi(apiServerURL()).StopSpace(cmd.Context(), args[0], true); err != nil {
				return err
			}
			fmt.Printf("Agent %s killed\n", args[0])
			return nil
		},
	}
}
