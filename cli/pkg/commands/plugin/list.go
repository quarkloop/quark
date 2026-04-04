package plugincmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed plugins with type and version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			pluginsDir, err := getPluginsDir()
			if err != nil {
				return err
			}
			plugins, err := svc.List(cmd.Context(), pluginsDir)
			if err != nil {
				return fmt.Errorf("list failed: %w", err)
			}
			if len(plugins) == 0 {
				fmt.Println("No plugins installed.")
				return nil
			}
			for _, p := range plugins {
				fmt.Printf("%s %s (%s)\n", p.Manifest.Name, p.Manifest.Version, p.Manifest.Type)
			}
			return nil
		},
	}
}
