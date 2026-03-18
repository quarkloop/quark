package registry

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/api-server/pkg/cli/config"
)

// ScaffoldCLI returns the "registry" command group.
func ScaffoldCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage the local agent/skill registry",
	}
	cmd.AddCommand(scaffoldCmd())
	return cmd
}

func scaffoldCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scaffold",
		Short: "Create example agent/skill definitions in ~/.quark/registry/",
		Long: `Creates a local registry scaffold at ~/.quark/registry/ with a default
supervisor agent and a basic tool skill so you can run spaces without
connecting to the remote quarkloop.com registry.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := api.NewClientApi(config.APIServerURL()).ScaffoldRegistry(cmd.Context()); err != nil {
				return err
			}
			fmt.Println("Registry scaffold complete.")
			return nil
		},
	}
}
