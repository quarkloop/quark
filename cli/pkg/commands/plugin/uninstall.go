package plugincmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/core/pkg/space"
)

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "uninstall <type-name>",
		Short:             "Uninstall a plugin",
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { return middleware.RequireSpace() },
		RunE: func(cmd *cobra.Command, args []string) error {
			pluginsDir, err := getPluginsDir()
			if err != nil {
				return err
			}
			if err := svc.Uninstall(cmd.Context(), args[0], pluginsDir); err != nil {
				return fmt.Errorf("uninstall failed: %w", err)
			}
			fmt.Printf("Uninstalled %s\n", args[0])

			spaceDir, err := space.FindRoot(".")
			if err == nil {
				if err := QuarkRemove(spaceDir, args[0]); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not update Quarkfile: %v\n", err)
				}
			}
			return nil
		},
	}
}
