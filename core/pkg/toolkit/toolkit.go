// Package toolkit provides shared bootstrapping for quark tool binaries.
//
// Every tool binary (bash, web-search, kb, space, agent) follows the same
// pattern: create a cobra root command, register subcommands, execute.
// This package captures that pattern so tool binaries stay thin.
package toolkit

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewToolCommand creates a standard cobra root command for a quark tool binary.
func NewToolCommand(name, description string) *cobra.Command {
	return &cobra.Command{
		Use:          name,
		Short:        fmt.Sprintf("quark %s — %s", name, description),
		SilenceUsage: true,
	}
}

// Execute runs the root command and exits with code 1 on error.
func Execute(root *cobra.Command) {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
