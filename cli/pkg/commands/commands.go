package commands

import (
	"github.com/spf13/cobra"

	activityCmd "github.com/quarkloop/cli/pkg/commands/activity"
	configCmd "github.com/quarkloop/cli/pkg/commands/config"
	"github.com/quarkloop/cli/pkg/commands/doctor"
	initCmd "github.com/quarkloop/cli/pkg/commands/init"
	kbCmd "github.com/quarkloop/cli/pkg/commands/kb"
	planCmd "github.com/quarkloop/cli/pkg/commands/plan"
	pluginCmd "github.com/quarkloop/cli/pkg/commands/plugin"
	"github.com/quarkloop/cli/pkg/commands/runtime"
	sessionCmd "github.com/quarkloop/cli/pkg/commands/session"
	"github.com/quarkloop/cli/pkg/commands/version"
)

func RegisterCommands(root *cobra.Command) {
	// Space Commands — agent lifecycle + space operations.
	addGroup(root, "space",
		runtime.RunCLI(),
		runtime.StopCLI(),
		runtime.InspectCLI(),
		initCmd.InitCLI(),
		doctor.DoctorCLI(),
		version.VersionCLI(),
	)

	// Data Commands — session, config, kb, plan, activity management.
	addGroup(root, "data",
		sessionCmd.NewSessionCommand(),
		configCmd.NewConfigCommand(),
		kbCmd.NewKBCommand(),
		planCmd.NewPlanCommand(),
		activityCmd.NewActivityCommand(),
	)

	// Management Commands — plugin manager and validation.
	addGroup(root, "management",
		pluginCmd.NewCommand(),
	)
}

func addGroup(root *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.GroupID = groupID
		root.AddCommand(c)
	}
}
