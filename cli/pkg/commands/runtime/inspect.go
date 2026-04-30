package runtimecmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	agentclient "github.com/quarkloop/agent/pkg/client"
	spacemodel "github.com/quarkloop/pkg/space"
	"github.com/quarkloop/supervisor/pkg/api"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

// InspectCLI returns the "inspect" command.
func InspectCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect",
		Short: "Show status and details of the current space and its agent",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			name, err := spacemodel.NameFromDir(cwd)
			if err != nil {
				return err
			}
			sup := supclient.New()
			info, err := sup.GetSpace(cmd.Context(), name)
			if err != nil {
				return err
			}
			fmt.Printf("Space:      %s\n", info.Name)
			if info.Version != "" {
				fmt.Printf("Version:    %s\n", info.Version)
			}
			fmt.Printf("Created:    %s\n", info.CreatedAt.Format(time.RFC3339))
			fmt.Printf("Updated:    %s\n", info.UpdatedAt.Format(time.RFC3339))

			agent, err := sup.AgentBySpace(cmd.Context(), name)
			if err != nil {
				if supclient.IsNotFound(err) {
					fmt.Println("Agent:      (not running)")
					return nil
				}
				return err
			}
			fmt.Printf("Agent ID:   %s\n", agent.ID)
			fmt.Printf("Status:     %s\n", agent.Status)
			if agent.PID != 0 {
				fmt.Printf("PID:        %d\n", agent.PID)
			}
			if agent.Port != 0 {
				fmt.Printf("Port:       %d\n", agent.Port)
			}
			if !agent.StartedAt.IsZero() {
				fmt.Printf("Started:    %s\n", agent.StartedAt.Format(time.RFC3339))
			}
			if agent.Uptime != "" {
				fmt.Printf("Uptime:     %s\n", agent.Uptime)
			}
			if agent.Status != api.AgentRunning {
				return nil
			}
			runtimeInfo, err := agentclient.New(agent.URL()).Info(cmd.Context())
			if err != nil {
				return nil
			}
			fmt.Printf("Provider:   %s\n", runtimeInfo.Provider)
			fmt.Printf("Model:      %s\n", runtimeInfo.Model)
			fmt.Printf("Mode:       %s\n", runtimeInfo.Mode)
			return nil
		},
	}
}
