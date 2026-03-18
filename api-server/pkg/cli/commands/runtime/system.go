package runtime

import "github.com/spf13/cobra"

// SystemCLI returns the "system" command group.
func SystemCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "API server management",
	}
	cmd.AddCommand(systemStatusCLI())
	return cmd
}
