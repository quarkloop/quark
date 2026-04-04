// Package config provides CLI commands for managing agent configuration.
package config

import (
	"fmt"

	"github.com/spf13/cobra"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"

	"github.com/quarkloop/cli/pkg/middleware"
)

func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage agent configuration values",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return middleware.RequireSpace()
		},
	}
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigListCmd())
	cmd.AddCommand(newConfigDeleteCmd())
	return cmd
}

func agentURL(cmd *cobra.Command) string {
	url, _ := cmd.Flags().GetString("agent-url")
	if url == "" {
		return "http://127.0.0.1:7100"
	}
	return url
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value from the agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "mode":
				client := agentclient.New(agentURL(cmd))
				mode, err := client.Mode(cmd.Context())
				if err != nil {
					return fmt.Errorf("get mode: %w", err)
				}
				fmt.Printf("mode: %s\n", mode.Mode)
				return nil
			default:
				return fmt.Errorf("unknown config key: %s", args[0])
			}
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value on the agent",
		Long:  "Set a config value via the agent's config store. Supported keys: mode.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "mode":
				client := agentclient.New(agentURL(cmd))
				resp, err := client.Chat(cmd.Context(), agentapi.ChatRequest{
					Message: "set mode to " + args[1],
					Mode:    args[1],
				})
				if err != nil {
					return fmt.Errorf("set mode: %w", err)
				}
				fmt.Printf("mode set to %s\n", resp.Mode)
				return nil
			default:
				return fmt.Errorf("unsupported config key: %s", args[0])
			}
		},
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List agent configuration values",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := agentclient.New(agentURL(cmd))
			info, err := client.Info(cmd.Context())
			if err != nil {
				return fmt.Errorf("list config: %w", err)
			}
			mode, err := client.Mode(cmd.Context())
			if err != nil {
				return fmt.Errorf("list config: %w", err)
			}
			stats, err := client.Stats(cmd.Context())
			if err != nil {
				return fmt.Errorf("list config: %w", err)
			}
			fmt.Printf("agent_id: %s\n", info.AgentID)
			fmt.Printf("provider: %s\n", info.Provider)
			fmt.Printf("model:    %s\n", info.Model)
			fmt.Printf("mode:     %s\n", mode.Mode)
			fmt.Printf("tools:    %v\n", info.Tools)
			for k, v := range stats {
				fmt.Printf("%s: %v\n", k, v)
			}
			return nil
		},
	}
}

func newConfigDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a config value",
		Long:  "Delete a config value — only works for resettable keys.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = agentURL(cmd)
			fmt.Println("config delete not yet exposed via HTTP — use agent API directly")
			return nil
		},
	}
}
