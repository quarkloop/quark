package cli

import (
	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/cli/commands"
	"github.com/quarkloop/cli/pkg/cli/config"
)

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:          "quark [OPTIONS] COMMAND",
		Short:        "Quark — the agent runtime",
		Long:         "Your agents. Your machine. Fully Autonomous.\n\nQuark is a runtime for autonomous agents.",
		SilenceUsage: true,
	}

	root.AddGroup(&cobra.Group{
		ID:    "space",
		Title: "Commands:",
	})
	root.AddGroup(&cobra.Group{
		ID:    "management",
		Title: "Management Commands:",
	})

	root.PersistentFlags().StringVar(
		&config.APIServerOverride,
		"api-server",
		"",
		`api-server base URL (overrides QUARK_API_SERVER env var, default "`+config.DefaultAPIServerAddr+`")`,
	)

	commands.RegisterCommands(root)

	return root
}
