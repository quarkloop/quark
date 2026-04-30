package kbcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	spacemodel "github.com/quarkloop/pkg/space"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

func newKBListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <namespace>",
		Short: "List all keys in a KB namespace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := spacemodel.CurrentName()
			if err != nil {
				return err
			}
			keys, err := supclient.New().KBList(cmd.Context(), name, args[0])
			if err != nil {
				return err
			}
			if len(keys) == 0 {
				fmt.Printf("No keys in namespace %s\n", args[0])
				return nil
			}
			for _, k := range keys {
				fmt.Println(k)
			}
			return nil
		},
	}
}
