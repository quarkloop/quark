package init

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/space"
)

// InitCLI returns the "init" command.
func InitCLI() *cobra.Command {
	var withPlugins bool

	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a new space directory with Quarkfile and default structure",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			if err := space.Init(absDir); err != nil {
				return err
			}
			if withPlugins {
				if err := installBuiltinPlugins(absDir); err != nil {
					return fmt.Errorf("install builtin plugins: %w", err)
				}
			}
			msg := "Space initialised in %s\n"
			if withPlugins {
				msg += "Builtin plugins installed (tool-bash, tool-read, tool-write, tool-web-search)\n"
			}
			fmt.Printf(msg+"Next steps:\n  1. Edit Quarkfile\n  2. quark run\n", absDir)
			return nil
		},
	}

	cmd.Flags().BoolVar(&withPlugins, "with-plugins", false, "Install builtin plugins into .quark/plugins/")
	return cmd
}

// installBuiltinPlugins copies builtin plugin directories from the quark repo
// into the space's .quark/plugins/ directory.
func installBuiltinPlugins(spaceDir string) error {
	_, srcFile, _, _ := runtime.Caller(0)
	// cli/pkg/commands/init/init.go → go up 4 levels to workspace root
	workspace := findWorkspace(srcFile)
	srcDir := filepath.Join(workspace, "plugins")

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read builtin plugins: %w", err)
	}
	destDir := filepath.Join(spaceDir, ".quark", "plugins")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dest := filepath.Join(destDir, e.Name())
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		if err := copyDir(filepath.Join(srcDir, e.Name()), dest); err != nil {
			return err
		}
	}
	return nil
}

// findWorkspace walks up from the current file until it finds a directory with go.work.
func findWorkspace(start string) string {
	dir := filepath.Dir(start)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(s)
			if err != nil {
				return err
			}
			if err := os.WriteFile(d, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
