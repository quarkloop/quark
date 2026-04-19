package configcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/quarkfile"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configuration keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, err := quarkfile.CurrentName()
			if err != nil {
				return err
			}
			keys, err := supclient.New().KBList(cmd.Context(), name, configNamespace)
			if err != nil {
				return err
			}
			if len(keys) == 0 {
				fmt.Println("No configuration values set")
				return nil
			}
			for _, k := range keys {
				fmt.Println(k)
			}
			return nil
		},
	}
}
