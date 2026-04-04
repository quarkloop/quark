package versioncmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/buildinfo"
)

// VersionCLI returns the "version" command.
func VersionCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("quark version %s\n", buildinfo.Version)
			return nil
		},
	}
}
