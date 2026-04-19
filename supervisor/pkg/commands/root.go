// Package commands defines the Cobra command tree for the supervisor binary.
package commands

import (
	"github.com/spf13/cobra"
)

// Init returns the root Cobra command for the supervisor binary.
func Init() *cobra.Command {
	root := &cobra.Command{
		Use:          "supervisor",
		Short:        "quark supervisor — manages spaces and provides the space API",
		SilenceUsage: true,
		Run: func(c *cobra.Command, args []string) {
			_ = c.Help()
		},
	}

	root.AddCommand(StartCmd())
	root.AddCommand(StopCmd())

	return root
}
