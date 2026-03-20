package space

import "github.com/spf13/cobra"

// SpaceCLI returns the "space" management command group.
// The day-to-day space commands (run, ps, stop, kill, inspect, logs, activity)
// live flat at the root for convenience, mirroring Docker's design.
// This command provides space management operations not covered by those aliases.
func SpaceCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "space",
		Short: "Manage spaces",
		Long:  "Manage space instances — list, remove, and inspect runtime stats.",
	}
	cmd.AddCommand(
		spaceLsCmd(),
		spaceRmCmd(),
		spaceStatsCmd(),
		spacePruneCmd(),
	)
	return cmd
}
