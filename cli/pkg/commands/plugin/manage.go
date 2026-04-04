package plugin

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/cli/pkg/plugin"
)

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "uninstall <type-name>",
		Short:             "Uninstall a plugin",
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { return middleware.RequireSpace() },
		RunE: func(cmd *cobra.Command, args []string) error {
			pluginsDir, err := getPluginsDir()
			if err != nil {
				return err
			}
			mgr := plugin.NewManager()
			if err := mgr.Uninstall(pluginsDir, args[0]); err != nil {
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
		Short: "List installed plugins with type and version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			pluginsDir, err := getPluginsDir()
			if err != nil {
				return err
			}
			mgr := plugin.NewManager()
			plugins, err := mgr.List(pluginsDir)
			if err != nil {
				return fmt.Errorf("list failed: %w", err)
			}
			if len(plugins) == 0 {
				fmt.Println("No plugins installed.")
				return nil
			}
			for _, p := range plugins {
				fmt.Printf("%s %s (%s)\n", p.Manifest.Name, p.Manifest.Version, p.Manifest.Type)
			}
			return nil
		},
	}
}

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <type-name>",
		Short: "Show plugin manifest and details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pluginsDir, err := getPluginsDir()
			if err != nil {
				return err
			}
			mgr := plugin.NewManager()
			p, err := mgr.Get(pluginsDir, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Name:        %s\n", p.Manifest.Name)
			fmt.Printf("Version:     %s\n", p.Manifest.Version)
			fmt.Printf("Type:        %s\n", p.Manifest.Type)
			fmt.Printf("Description: %s\n", p.Manifest.Description)
			fmt.Printf("Author:      %s\n", p.Manifest.Author)
			fmt.Printf("License:     %s\n", p.Manifest.License)
			if p.Manifest.Repository != "" {
				fmt.Printf("Repository:  %s\n", p.Manifest.Repository)
			}
			if len(p.Manifest.Tools) > 0 {
				fmt.Printf("Tools:       %s\n", strings.Join(p.Manifest.Tools, ", "))
			}
			if len(p.Manifest.Skills) > 0 {
				fmt.Printf("Skills:      %s\n", strings.Join(p.Manifest.Skills, ", "))
			}
			if p.Manifest.Prompt != "" {
				fmt.Printf("Prompt:      %s\n", p.Manifest.Prompt)
			}
			return nil
		},
	}
}

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build [dir]",
		Short: "Validate a plugin from source",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			man, err := plugin.LoadLocal(dir)
			if err != nil {
				return fmt.Errorf("validate failed: %w", err)
			}
			fmt.Printf("Plugin %s %s (%s) is valid\n", man.Name, man.Version, man.Type)
			return nil
		},
	}
}
