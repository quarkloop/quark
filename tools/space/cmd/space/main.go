// space is the quark space CLI tool.
//
// It manages space directory filesystem operations: scaffolding, locking,
// validating, and managing agents/tools/KB entries in a Quarkfile.
// The api-server proxies these operations over HTTP; this binary is the
// single implementation of the filesystem side.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/core/pkg/toolkit"
	"github.com/quarkloop/tools/space/pkg/repo"
)

func main() {
	root := toolkit.NewToolCommand("space", "manage space directory filesystem operations")

	root.AddCommand(initCmd())
	root.AddCommand(lockCmd())
	root.AddCommand(validateCmd())

	toolkit.Execute(root)
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a new space directory with Quarkfile and default structure",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			if err := repo.Init(dir); err != nil {
				return err
			}
			fmt.Printf("Space initialised in %s\n", dir)
			fmt.Println("Next steps:")
			fmt.Println("  1. Edit Quarkfile")
			fmt.Println("  2. space lock")
			fmt.Println("  3. quark run")
			return nil
		},
	}
}

func lockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lock [dir]",
		Short: "Resolve refs and write .quark/lock.yaml",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			return repo.Lock(dir)
		},
	}
}

func validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [dir]",
		Short: "Validate the Quarkfile and lock file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			return repo.Validate(dir)
		},
	}
}
