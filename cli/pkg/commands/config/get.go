package configcmd

import (
	"github.com/spf13/cobra"

	spacemodel "github.com/quarkloop/pkg/space"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Read a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := spacemodel.CurrentName()
			if err != nil {
				return err
			}
			val, err := supclient.New().KBGet(cmd.Context(), name, configNamespace, args[0])
			if err != nil {
				return err
			}
			cmd.Print(string(val))
			return nil
		},
	}
}
