// Package dualmode provides shared helpers for resolving whether a CLI
// command should operate in local (filesystem) or HTTP (agent) mode.
package resolve

import (
	"github.com/spf13/cobra"

	"github.com/quarkloop/core/pkg/space"
)

// AgentURL reads the --agent-url flag from the command hierarchy.
// Returns an empty string when the flag is absent or unset, indicating
// local mode should be used.
func AgentURL(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	f := cmd.Flag("agent-url")
	if f == nil {
		f = cmd.Root().PersistentFlags().Lookup("agent-url")
	}
	if f == nil {
		return ""
	}
	return f.Value.String()
}

// SpaceDir finds the space root directory by walking up from the current
// working directory until a Quarkfile is found. Returns an error if no
// Quarkfile exists in any ancestor.
func SpaceDir() (string, error) {
	return space.FindRoot(".")
}
