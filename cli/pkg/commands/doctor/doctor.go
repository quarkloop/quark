// Package doctor provides CLI commands for diagnosing space health.
package doctor

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/cli/pkg/space"
)

// DoctorCLI returns the "doctor" command.
func DoctorCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose space config and plugin health",
		Args:  cobra.MaximumNArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return middleware.RequireSpace()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			if err := space.Validate(dir); err != nil {
				return fmt.Errorf("doctor check failed: %w", err)
			}
			fmt.Println("All checks passed.")
			return nil
		},
	}
}
