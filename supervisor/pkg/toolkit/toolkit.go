// Package toolkit provides shared CLI bootstrap helpers for quark tool binaries.
package toolkit

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewToolCommand creates a root cobra command for a quark tool binary.
func NewToolCommand(name, description string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: description,
		// Silence default usage output on errors; each subcommand
		// is responsible for its own usage messages.
		SilenceUsage: true,
	}
}

// Execute runs the given command and exits with a non-zero status on error.
func Execute(cmd *cobra.Command) {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
