package lock

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	spacerepo "github.com/quarkloop/tools/space/pkg/repo"
)

// LockCLI returns the "lock" command.
func LockCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "lock [dir]",
		Short: "Resolve all agent/tool refs and write .quark/lock.yaml",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			if err := spacerepo.Lock(absDir); err != nil {
				return err
			}
			fmt.Println("Lock file written → .quark/lock.yaml")
			return nil
		},
	}
}
