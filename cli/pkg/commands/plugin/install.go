package plugincmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/core/pkg/space"
)

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <ref>",
		Short: "Install a plugin from the registry, a git URL, or a local directory",
		Long: `Install a plugin into .quark/plugins/ of the current space.

  quark plugin install tool-bash                  # registry plugin name
  quark plugin install github.com/user/tool-name  # git clone from URL
  quark plugin install ./local-plugin/            # from local directory`,
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { return middleware.RequireSpace() },
		RunE: func(cmd *cobra.Command, args []string) error {
			pluginsDir, err := getPluginsDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(pluginsDir, 0755); err != nil {
				return fmt.Errorf("create plugins dir: %w", err)
			}
			man, err := svc.Install(cmd.Context(), args[0], pluginsDir)
			if err != nil {
				return err
			}
			fmt.Printf("Installed %s %s (%s)\n", man.Name, man.Version, man.Type)

			spaceDir, err := space.FindRoot(".")
			if err == nil {
				if err := QuarkAdd(spaceDir, args[0]); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not update Quarkfile: %v\n", err)
				}
			}
			return nil
		},
	}
}

func getPluginsDir() (string, error) {
	spaceDir, err := space.FindRoot(".")
	if err != nil {
		return "", err
	}
	return filepath.Join(spaceDir, ".quark", "plugins"), nil
}
