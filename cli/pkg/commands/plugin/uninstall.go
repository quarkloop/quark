package plugincmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/quarkfile"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Uninstall a plugin from the current space (via supervisor API)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := quarkfile.CurrentName()
			if err != nil {
				return err
			}
			if err := supclient.New().UninstallPlugin(cmd.Context(), name, args[0]); err != nil {
				return fmt.Errorf("uninstall failed: %w", err)
			}
			fmt.Printf("Uninstalled %s\n", args[0])
			return nil
		},
	}
}
