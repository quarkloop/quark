package plugincmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/middleware"
)

func newUpdateCmd() *cobra.Command {
	var updateAll bool
	cmd := &cobra.Command{
		Use:               "update [name]",
		Short:             "Update an installed plugin from its remote source",
		Args:              cobra.MaximumNArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { return middleware.RequireSpace() },
		RunE: func(cmd *cobra.Command, args []string) error {
			pluginsDir, err := getPluginsDir()
			if err != nil {
				return err
			}

			if updateAll {
				plugins, err := svc.List(cmd.Context(), pluginsDir)
				if err != nil {
					return fmt.Errorf("list plugins: %w", err)
				}
				for _, p := range plugins {
					man, err := svc.Update(cmd.Context(), p.Manifest.TypeDirName(), pluginsDir)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: update %s failed: %v\n", p.Manifest.Name, err)
						continue
					}
					fmt.Printf("Updated %s to %s\n", man.Name, man.Version)
				}
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("provide a plugin name or use --all")
			}
			man, err := svc.Update(cmd.Context(), args[0], pluginsDir)
			if err != nil {
				return fmt.Errorf("update failed: %w", err)
			}
			fmt.Printf("Updated %s to %s\n", man.Name, man.Version)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&updateAll, "all", "a", false, "Update all installed plugins")
	return cmd
}
