// Package initcmd provides the `quark init` command.
package initcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/quarkfile"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

// InitCLI returns the "init" command. It writes a Quarkfile to the user's
// working directory (if missing) and registers the space with the
// supervisor.
func InitCLI() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a Quarkfile and register the space with the supervisor",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			abs, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(abs, 0o755); err != nil {
				return fmt.Errorf("create dir: %w", err)
			}

			if !quarkfile.Exists(abs) {
				spaceName := name
				if spaceName == "" {
					spaceName = filepath.Base(abs)
				}
				if err := quarkfile.Write(abs, quarkfile.DefaultTemplate(spaceName)); err != nil {
					return err
				}
			}

			data, err := quarkfile.Read(abs)
			if err != nil {
				return err
			}
			spaceName, err := quarkfile.Name(data)
			if err != nil {
				return err
			}

			sup := supclient.New()
			info, err := sup.CreateSpace(cmd.Context(), spaceName, data)
			if err != nil {
				if supclient.IsConflict(err) {
					fmt.Printf("Space %q is already registered.\n", spaceName)
					return nil
				}
				return err
			}
			fmt.Printf("Space initialised: %s (v%d)\n", info.Name, info.Version)
			fmt.Println("Next steps:")
			fmt.Println("  1. Edit Quarkfile")
			fmt.Println("  2. quark plugin install <ref>")
			fmt.Println("  3. quark run")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Space name (defaults to directory name)")
	return cmd
}
