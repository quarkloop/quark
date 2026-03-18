package lock

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/api-server/pkg/cli/config"
)

// LockCLI returns the "lock" command.
func LockCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "lock [dir]",
		Short: "Resolve all agent/skill refs and write .quark/lock.yaml",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			if err := api.NewClientApi(config.APIServerURL()).LockRepo(cmd.Context(), absDir); err != nil {
				return err
			}
			fmt.Println("Lock file written → .quark/lock.yaml")
			return nil
		},
	}
}
