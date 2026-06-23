package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteSystemFlag string

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete nodes by names",
	Long: `Delete nodes.

Examples:
  # Delete a system
  quarkctl delete system monitor -n alice

  # Delete (undeploy) a system (alias)
  quarkctl delete systems monitor -n alice`,
	Args: cobra.NoArgs,
}

var deleteSystemCmd = &cobra.Command{
	Use:   "system NAME",
	Short: "Delete a system (undeploy)",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeleteSystem,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.AddCommand(deleteSystemCmd)
}

func runDeleteSystem(cmd *cobra.Command, args []string) error {
	ns, err := requireNamespace()
	if err != nil {
		return err
	}
	c := newClient()
	ctx, cancel := ctx()
	defer cancel()
	if err := c.DeleteSystem(ctx, args[0], ns); err != nil {
		return newPrinter().PrintError(err)
	}
	fmt.Printf("✓ System %s/%s deleted.\n", ns, args[0])
	return nil
}
