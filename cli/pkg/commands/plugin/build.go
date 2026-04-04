package plugincmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build [dir]",
		Short: "Build a plugin from source (validate manifest and compile Go binaries)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			man, err := svc.Build(cmd.Context(), dir)
			if err != nil {
				return fmt.Errorf("build failed: %w", err)
			}
			fmt.Printf("Plugin %s %s (%s) is valid\n", man.Name, man.Version, man.Type)
			return nil
		},
	}
}
