package space

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/api-server/pkg/cli/config"
)

// spacePruneCmd removes all stopped and failed space records from the api-server.
func spacePruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Remove all stopped and failed spaces",
		Long:  "Remove all space records that are in stopped or failed state.\nRunning, starting, and stopping spaces are not affected.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			pruned, err := api.NewClientApi(config.APIServerURL()).PruneSpaces(cmd.Context())
			if err != nil {
				return err
			}
			if len(pruned) == 0 {
				fmt.Println("Nothing to prune.")
				return nil
			}
			for _, id := range pruned {
				fmt.Printf("Removed %s\n", id)
			}
			fmt.Printf("\n%d space(s) removed.\n", len(pruned))
			return nil
		},
	}
}
