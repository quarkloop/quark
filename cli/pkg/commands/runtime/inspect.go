package runtimecmd

import (
	"time"

	"github.com/spf13/cobra"

	agentclient "github.com/quarkloop/agent-client"
	"github.com/quarkloop/agent/pkg/infra/term"
)

// InspectCLI returns the "inspect" command.
func InspectCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <agent-url>",
		Short: "Show full details of a running agent",
		Long: `Connect to a running agent and display its status.

Example:
  quark inspect http://127.0.0.1:7100`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentURL := args[0]
			client := agentclient.New(agentURL)

			health, err := client.Health(cmd.Context())
			if err != nil {
				return err
			}
			mode, err := client.Mode(cmd.Context())
			if err != nil {
				return err
			}

			row := term.SpaceRow{
				ID:        health.AgentID,
				Name:      health.AgentID,
				Status:    health.Status,
				Mode:      mode.Mode,
				Port:      parsePort(agentURL),
				Dir:       agentURL,
				CreatedAt: time.Now(),
			}
			term.PrintSpaceDetail(row)
			return nil
		},
	}
	return cmd
}
