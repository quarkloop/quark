package configcmd

import (
	"github.com/spf13/cobra"

	spacemodel "github.com/quarkloop/pkg/space"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Write a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := spacemodel.CurrentName()
			if err != nil {
				return err
			}
			return supclient.New().KBSet(cmd.Context(), name, configNamespace, args[0], []byte(args[1]))
		},
	}
}
