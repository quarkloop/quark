// Package plugin provides the root command for plugin management.
package plugincmd

import (
	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/plugin"
)

// svc is the shared plugin service instance.
var svc plugin.Service = plugin.NewLocalService()

// NewCommand creates the plugin subcommand tree.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage agent plugins",
	}

	cmd.AddCommand(newSearchCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newUninstallCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newBuildCmd())

	return cmd
}
