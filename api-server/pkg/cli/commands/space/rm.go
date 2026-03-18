package space

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/api-server/pkg/cli/config"
)

// spaceRmCmd removes a stopped space record from the api-server.
// The space must already be stopped; use `quark stop` or `quark kill` first.
func spaceRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a stopped space",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := api.NewClientApi(config.APIServerURL()).DeleteSpace(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Removed space %s\n", args[0])
			return nil
		},
	}
}
