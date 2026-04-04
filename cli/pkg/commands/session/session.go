// Package session provides CLI commands for managing agent sessions.
package session

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"

	"github.com/quarkloop/cli/pkg/middleware"
)

func NewSessionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage agent sessions",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return middleware.RequireSpace()
		},
	}
	cmd.AddCommand(newSessionCreateCmd())
	cmd.AddCommand(newSessionGetCmd())
	cmd.AddCommand(newSessionDeleteCmd())
	cmd.AddCommand(newSessionListCmd())
	return cmd
}

func agentURLFromFlags(cmd *cobra.Command) string {
	url, _ := cmd.Flags().GetString("agent-url")
	if url == "" {
		return "http://127.0.0.1:7100"
	}
	return url
}

func newSessionCreateCmd() *cobra.Command {
	var sessType, title string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new session",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := agentclient.New(agentURLFromFlags(cmd))
			req := agentapi.CreateSessionRequest{
				Type:  agentapi.SessionType(sessType),
				Title: title,
			}
			resp, err := client.CreateSession(cmd.Context(), req)
			if err != nil {
				return fmt.Errorf("create session: %w", err)
			}
			fmt.Printf("Session created: %s\n", resp.Session.Key)
			return nil
		},
	}
	cmd.Flags().StringVar(&sessType, "type", "chat", "Session type (chat|subagent|cron)")
	cmd.Flags().StringVar(&title, "title", "", "Session title")
	return cmd
}

func newSessionGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <session-key>",
		Short: "Get a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := agentclient.New(agentURLFromFlags(cmd))
			session, err := client.Session(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("get session: %w", err)
			}
			data, _ := json.MarshalIndent(session, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

func newSessionDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <session-key>",
		Short: "Delete a session (cannot delete main)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := agentclient.New(agentURLFromFlags(cmd))
			if err := client.DeleteSession(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("delete session: %w", err)
			}
			fmt.Printf("Session deleted: %s\n", args[0])
			return nil
		},
	}
}

func newSessionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := agentclient.New(agentURLFromFlags(cmd))
			sessions, err := client.Sessions(cmd.Context())
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}
			if len(sessions) == 0 {
				fmt.Println("No sessions.")
				return nil
			}
			for _, s := range sessions {
				fmt.Printf("%-12s %s  %s\n", s.Type, s.Key, s.Title)
			}
			return nil
		},
	}
}
