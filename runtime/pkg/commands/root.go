package commands

import (
	"github.com/spf13/cobra"
)

// Init returns the root cobra command for the agent binary.
func Init() *cobra.Command {
	root := &cobra.Command{
		Use:          "agent",
		Short:        "quark agent — agent runtime process",
		SilenceUsage: true,
		Run: func(c *cobra.Command, args []string) {
			_ = c.Help()
		},
	}

	root.AddCommand(Start())

	return root
}
