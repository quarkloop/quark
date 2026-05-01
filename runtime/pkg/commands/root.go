package commands

import (
	"github.com/spf13/cobra"
)

// Init returns the root cobra command for the runtime binary.
func Init() *cobra.Command {
	root := &cobra.Command{
		Use:          "runtime",
		Short:        "quark runtime process",
		SilenceUsage: true,
		Run: func(c *cobra.Command, args []string) {
			_ = c.Help()
		},
	}

	root.AddCommand(Start())

	return root
}
