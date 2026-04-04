// Package configcmd provides CLI commands for managing agent configuration.
package configcmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/config"
	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/cli/pkg/resolve"
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

func resolveClient(cmd *cobra.Command) (*config.Client, error) {
	if url := resolve.AgentURL(cmd); url != "" {
		return config.NewHTTP(url), nil
	}
	dir, err := resolve.SpaceDir()
	if err != nil {
		return nil, err
	}
	return config.NewLocal(dir)
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			val, err := c.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("get config: %w", err)
			}
			fmt.Printf("%s: %s\n", args[0], val)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			if err := c.Set(cmd.Context(), args[0], args[1]); err != nil {
				return fmt.Errorf("set config: %w", err)
			}
			fmt.Printf("%s set to %s\n", args[0], args[1])
			return nil
		},
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List agent configuration values",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			vals, err := c.List(cmd.Context())
			if err != nil {
				return fmt.Errorf("list config: %w", err)
			}
			if len(vals) == 0 {
				fmt.Println("No configuration values.")
				return nil
			}
			keys := make([]string, 0, len(vals))
			for k := range vals {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("%s: %s\n", k, vals[k])
			}
			return nil
		},
	}
}

func newConfigDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			if err := c.Delete(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("delete config: %w", err)
			}
			fmt.Printf("%s deleted\n", args[0])
			return nil
		},
	}
}
