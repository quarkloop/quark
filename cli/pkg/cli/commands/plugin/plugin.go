// Package plugin provides CLI commands for managing plugins.
package plugin

import (
	"fmt"
	"path/filepath"

	"github.com/quarkloop/agent/pkg/plugin"
	"github.com/spf13/cobra"
)

// NewCommand creates the plugin subcommand tree.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage agent plugins",
	}

	cmd.AddCommand(newSearchCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newUninstallCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newInfoCmd())

	return cmd
}

func newSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search the plugin hub",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := plugin.NewHubClient("")
			results, err := client.Search(args[0])
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}
			if len(results) == 0 {
				fmt.Println("No plugins found.")
				return nil
			}
			for _, r := range results {
				fmt.Printf("%s (%s) — %s\n", r.Name, r.Version, r.Description)
			}
			return nil
		},
	}
}

func newInstallCmd() *cobra.Command {
	var hubURL string
	cmd := &cobra.Command{
		Use:   "install <name[@version] | ./path>",
		Short: "Install a plugin from the hub or a local directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := plugin.NewManager()

			// Local install.
			if filepath.IsAbs(args[0]) || args[0][0] == '.' {
				loaded, err := mgr.Install(args[0], nil)
				if err != nil {
					return fmt.Errorf("install failed: %w", err)
				}
				fmt.Printf("Installed %s %s (%s)\n", loaded.Manifest.Name, loaded.Manifest.Version, loaded.Manifest.Type)
				return nil
			}

			// Hub install.
			name := args[0]
			version := ""
			if idx := len(args[0]); idx > 0 {
				for i, c := range args[0] {
					if c == '@' {
						name = args[0][:i]
						version = args[0][i+1:]
						break
					}
				}
			}

			client := plugin.NewHubClient(hubURL)
			loaded, err := mgr.InstallFromHub(name, version, client, nil)
			if err != nil {
				return fmt.Errorf("install from hub failed: %w", err)
			}
			fmt.Printf("Installed %s %s (%s)\n", loaded.Manifest.Name, loaded.Manifest.Version, loaded.Manifest.Type)
			return nil
		},
	}
	cmd.Flags().StringVar(&hubURL, "hub", "", "Plugin hub URL")
	return cmd
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Uninstall a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := plugin.NewManager()
			if err := mgr.Uninstall(args[0]); err != nil {
				return fmt.Errorf("uninstall failed: %w", err)
			}
			fmt.Printf("Uninstalled %s\n", args[0])
			return nil
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := plugin.NewManager()
			plugins := mgr.List()
			if len(plugins) == 0 {
				fmt.Println("No plugins installed.")
				return nil
			}
			for _, p := range plugins {
				fmt.Printf("%s %s (%s) — %s\n", p.Manifest.Name, p.Manifest.Version, p.Manifest.Type, p.Status)
			}
			return nil
		},
	}
}

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show plugin details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := plugin.NewManager()
			p, err := mgr.Get(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Name:        %s\n", p.Manifest.Name)
			fmt.Printf("Version:     %s\n", p.Manifest.Version)
			fmt.Printf("Type:        %s\n", p.Manifest.Type)
			fmt.Printf("Description: %s\n", p.Manifest.Description)
			fmt.Printf("Author:      %s\n", p.Manifest.Author)
			fmt.Printf("License:     %s\n", p.Manifest.License)
			fmt.Printf("Status:      %s\n", p.Status)
			return nil
		},
	}
}
