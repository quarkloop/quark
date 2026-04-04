// Package plugin provides the root command for plugin management.
package plugin

import "github.com/spf13/cobra"

// NewCommand creates the plugin subcommand tree.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage agent plugins",
	}

	cmd.AddCommand(newSearchCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newUninstallCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newBuildCmd())

	return cmd
}
