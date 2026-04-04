// Package kbcmd provides CLI commands for managing the agent knowledge base.
package kbcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/kb"
	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/cli/pkg/resolve"
)

func NewKBCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kb",
		Short: "Manage the agent knowledge base",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return middleware.RequireSpace()
		},
	}
	cmd.AddCommand(kbGetCmd())
	cmd.AddCommand(kbSetCmd())
	cmd.AddCommand(kbDeleteCmd())
	cmd.AddCommand(kbListCmd())
	return cmd
}

func resolveClient(cmd *cobra.Command) (*kb.Client, error) {
	if url := resolve.AgentURL(cmd); url != "" {
		return kb.NewHTTP(url, nil), nil
	}
	dir, err := resolve.SpaceDir()
	if err != nil {
		return nil, err
	}
	return kb.NewLocal(dir)
}

func parseKey(s string) (string, string, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid namespace/key %q — must be <namespace>/<key>", s)
	}
	return parts[0], parts[1], nil
}
