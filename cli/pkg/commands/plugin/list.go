package plugincmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/quarkfile"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

func newListCmd() *cobra.Command {
	var typeFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins (via supervisor API)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, err := quarkfile.CurrentName()
			if err != nil {
				return err
			}
			plugins, err := supclient.New().ListPlugins(cmd.Context(), name, typeFilter)
			if err != nil {
				return fmt.Errorf("list failed: %w", err)
			}
			if len(plugins) == 0 {
				fmt.Println("No plugins installed.")
				return nil
			}
			for _, p := range plugins {
				fmt.Printf("%-24s %-10s %-10s %s\n", p.Name, p.Version, p.Type, p.Description)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by plugin type (tool, provider, agent, skill)")
	return cmd
}
