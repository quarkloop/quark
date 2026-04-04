// Package session provides CLI commands for managing agent sessions.
package sessioncmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	agentapi "github.com/quarkloop/agent-api"
	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/cli/pkg/resolve"
	sessioncli "github.com/quarkloop/cli/pkg/session"
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

func resolveClient(cmd *cobra.Command) (*sessioncli.Client, error) {
	if url := resolve.AgentURL(cmd); url != "" {
		return sessioncli.NewHTTP(url), nil
	}
	dir, err := resolve.SpaceDir()
	if err != nil {
		return nil, err
	}
	return sessioncli.NewLocal(dir)
}

func newSessionCreateCmd() *cobra.Command {
	var sessType, title string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new session",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			req := agentapi.CreateSessionRequest{
				Type:  agentapi.SessionType(sessType),
				Title: title,
			}
			resp, err := c.Create(cmd.Context(), req)
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
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			sess, err := c.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("get session: %w", err)
			}
			data, _ := json.MarshalIndent(sess, "", "  ")
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
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			if err := c.Delete(cmd.Context(), args[0]); err != nil {
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
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			sessions, err := c.List(cmd.Context())
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
