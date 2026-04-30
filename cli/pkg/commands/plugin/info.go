package plugincmd

import (
	"fmt"

	"github.com/spf13/cobra"

	spacemodel "github.com/quarkloop/pkg/space"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show installed plugin details (via supervisor API)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := spacemodel.CurrentName()
			if err != nil {
				return err
			}
			p, err := supclient.New().GetPlugin(cmd.Context(), name, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Name:        %s\n", p.Name)
			fmt.Printf("Version:     %s\n", p.Version)
			fmt.Printf("Type:        %s\n", p.Type)
			fmt.Printf("Mode:        %s\n", p.Mode)
			fmt.Printf("Description: %s\n", p.Description)
			return nil
		},
	}
}
