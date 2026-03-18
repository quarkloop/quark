package init

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/cli/pkg/cli/config"
)

// InitCLI returns the "init" command.
func InitCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a new space repository",
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
			if err := api.NewClientApi(config.APIServerURL()).InitRepo(cmd.Context(), absDir); err != nil {
				return err
			}
			fmt.Println("Done. Next steps:\n  1. Edit Quarkfile\n  2. quark lock\n  3. quark run")
			return nil
		},
	}
}
