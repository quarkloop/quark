package plugincmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

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
			p, err := svc.Info(cmd.Context(), args[0], pluginsDir)
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
