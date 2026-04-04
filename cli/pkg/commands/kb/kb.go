// Package kb provides CLI commands for managing the agent knowledge base.
// Uses the kbclient which supports both local filesystem and HTTP modes.
package kb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/kbclient"
	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/cli/pkg/quarkfile"
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

func resolveSpaceDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	if !quarkfile.Exists(abs) {
		return "", fmt.Errorf("no Quarkfile found in %s (navigate into a space directory first)", abs)
	}
	return abs, nil
}

func client(cmd *cobra.Command) (*kbclient.Client, error) {
	if url := flagVal(cmd, "agent-url"); url != "" {
		return kbclient.NewHTTP(url, nil), nil
	}
	dir, err := resolveSpaceDir()
	if err != nil {
		return nil, err
	}
	return kbclient.NewLocal(dir)
}

func flagVal(cmd *cobra.Command, name string) string {
	if cmd == nil {
		return ""
	}
	f := cmd.Flag(name)
	if f == nil {
		f = cmd.Root().PersistentFlags().Lookup(name)
	}
	if f == nil {
		return ""
	}
	return f.Value.String()
}

func kbGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <namespace/key>",
		Short: "Read a KB entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, key, err := splitNsKey(args[0])
			if err != nil {
				return err
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			val, err := c.Get(cmd.Context(), ns, key)
			if err != nil {
				return err
			}
			fmt.Print(string(val))
			return nil
		},
	}
}

func kbSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <namespace/key> <value|@file>",
		Short: "Write a KB entry (use @<path> to read from file)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, key, err := splitNsKey(args[0])
			if err != nil {
				return err
			}
			var value []byte
			if strings.HasPrefix(args[1], "@") {
				value, err = os.ReadFile(args[1][1:])
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
			} else {
				value = []byte(args[1])
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			if err := c.Set(cmd.Context(), ns, key, value); err != nil {
				return fmt.Errorf("kb set: %w", err)
			}
			fmt.Printf("KB %s set\n", args[0])
			return nil
		},
	}
}

func kbDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <namespace/key>",
		Short: "Delete a KB entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, key, err := splitNsKey(args[0])
			if err != nil {
				return err
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			return c.Delete(cmd.Context(), ns, key)
		},
	}
}

func kbListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all keys in a KB namespace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			keys, err := c.List(cmd.Context(), args[0])
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

func splitNsKey(s string) (string, string, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid namespace/key %q — must be <namespace>/<key>", s)
	}
	return parts[0], parts[1], nil
}
