package validate

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/api-server/pkg/cli/config"
)

// ValidateCLI returns the "validate" command.
func ValidateCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [dir]",
		Short: "Validate the Quarkfile and lock file",
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
			if err := api.NewClientApi(config.APIServerURL()).ValidateRepo(cmd.Context(), absDir); err != nil {
				return err
			}
			fmt.Println("All checks passed.")
			return nil
		},
	}
}
