package commands

import (
	"github.com/spf13/cobra"

	initCmd "github.com/quarkloop/api-server/pkg/cli/commands/init"
	"github.com/quarkloop/api-server/pkg/cli/commands/lock"
	"github.com/quarkloop/api-server/pkg/cli/commands/registry"
	"github.com/quarkloop/api-server/pkg/cli/commands/runtime"
	"github.com/quarkloop/api-server/pkg/cli/commands/space"
	"github.com/quarkloop/api-server/pkg/cli/commands/validate"
	"github.com/quarkloop/api-server/pkg/cli/commands/version"
)

func RegisterCommands(root *cobra.Command) {
	// Commands — space lifecycle + repo commands share this group.
	addGroup(root, "space",
		runtime.RunCLI(),
		runtime.PsCLI(),
		runtime.StopCLI(),
		runtime.KillCLI(),
		runtime.InspectCLI(),
		runtime.LogsCLI(),
		runtime.EventsCLI(),
		initCmd.InitCLI(),
		lock.LockCLI(),
		validate.ValidateCLI(),
		version.VersionCLI(),
	)

	// Management Commands — subcommand groups.
	addGroup(root, "management",
		space.SpaceCLI(),
		registry.ScaffoldCLI(),
		runtime.SystemCLI(),
	)
}

// addGroup stamps groupID onto each command and adds it to root.
func addGroup(root *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.GroupID = groupID
		root.AddCommand(c)
	}
}
