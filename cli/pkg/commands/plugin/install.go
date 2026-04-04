package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/cli/pkg/plugin"
	"github.com/quarkloop/core/pkg/space"
)

const (
	registryOwner = "quarkloop"
	registryRepo  = "plugins"
	registryRoot  = "plugins"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <ref>",
		Short: "Install a plugin from the registry, a git URL, or a local directory",
		Long: `Install a plugin into .quark/plugins/ of the current space.

  quark plugin install tool-bash                  # registry plugin name
  quark plugin install github.com/user/tool-name  # git clone from URL
  quark plugin install ./local-plugin/            # from local directory`,
		Args: cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return middleware.RequireSpace()
		},
		RunE: runInstall,
	}
	return cmd
}

func runInstall(cmd *cobra.Command, args []string) error {
	ref := args[0]
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("create plugins dir: %w", err)
	}

	var srcDir, tmpDir string

	switch {
	case plugin.IsLocalPath(ref):
		srcDir, err = filepath.Abs(ref)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}
	case plugin.IsGitURL(ref):
		tmpDir, srcDir, err = cloneSingle(ref, pluginsDir)
		if err != nil {
			return fmt.Errorf("clone plugin: %w", err)
		}
		defer os.RemoveAll(tmpDir)
	default:
		tmpDir, srcDir, err = cloneFromRegistry(ref, pluginsDir)
		if err != nil {
			return fmt.Errorf("install %q from registry: %w", ref, err)
		}
		defer os.RemoveAll(tmpDir)
	}

	dest := filepath.Join(pluginsDir, plugin.DeriveName(ref))
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("plugin %s already exists — uninstall first", plugin.DeriveName(ref))
	}

	if err := plugin.CopyDir(srcDir, dest); err != nil {
		return fmt.Errorf("copy plugin: %w", err)
	}

	man, err := plugin.LoadLocal(dest)
	if err != nil {
		os.RemoveAll(dest)
		return fmt.Errorf("validate plugin: %w", err)
	}

	fmt.Printf("Installed %s %s (%s)\n", man.Name, man.Version, man.Type)
	return nil
}

// cloneSingle clones a single git repo into a temp dir under pluginsDir.
// Returns (tmpDir, srcDir, error). Caller should defer os.RemoveAll(tmpDir).
func cloneSingle(url, pluginsDir string) (tmpDir, srcDir string, err error) {
	tmpDir, err = os.MkdirTemp(pluginsDir, ".temp-")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}

	_, err = git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL:          url,
		SingleBranch: true,
		Tags:         git.NoTags,
		Depth:        1,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("git clone: %w", err)
	}

	if err := plugin.FixFileModes(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("set file modes: %w", err)
	}

	return tmpDir, tmpDir, nil
}

// cloneFromRegistry clones the registry monorepo to a temp dir,
// locates the requested plugin in plugins/<name>/, and returns it.
func cloneFromRegistry(name, pluginsDir string) (tmpDir, srcDir string, err error) {
	tmpDir, err = os.MkdirTemp(pluginsDir, ".temp-")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}

	url := plugin.ResolveRegistryURL()
	_, err = git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL:          url,
		SingleBranch: true,
		Tags:         git.NoTags,
		Depth:        1,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("clone registry: %w", err)
	}

	if err := plugin.FixFileModes(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("set file modes: %w", err)
	}

	srcDir = filepath.Join(tmpDir, registryRoot, name)
	if _, err := os.Stat(srcDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("plugin %q not found in registry", name)
	}

	return tmpDir, srcDir, nil
}

func getPluginsDir() (string, error) {
	spaceDir, err := space.FindRoot(".")
	if err != nil {
		return "", err
	}
	return filepath.Join(spaceDir, ".quark", "plugins"), nil
}
