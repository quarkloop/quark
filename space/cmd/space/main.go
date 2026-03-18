// space is the quark space CLI tool.
//
// It manages space directory filesystem operations: scaffolding, locking,
// validating, and managing agents/skills/KB entries in a Quarkfile.
// The api-server proxies these operations over HTTP; this binary is the
// single implementation of the filesystem side.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/quarkloop/space/pkg/repo"
)

func main() {
	root := &cobra.Command{
		Use:          "space",
		Short:        "quark space — manage space directory filesystem operations",
		SilenceUsage: true,
	}

	root.AddCommand(initCmd())
	root.AddCommand(lockCmd())
	root.AddCommand(validateCmd())
	root.AddCommand(scaffoldRegistryCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
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
		Short: "Resolve all agent/skill refs and write .quark/lock.yaml",
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

func scaffoldRegistryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scaffold-registry",
		Short: "Seed ~/.quark/registry/ with built-in agent and skill definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return repo.ScaffoldRegistry()
		},
	}
}
