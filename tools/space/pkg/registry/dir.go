package registry

import (
	"os"
	"path/filepath"
)

// localRegistryDir is where local agent/skill definitions are stored.
// ~/.quark/registry/agents/<name>/<version>.yaml
// ~/.quark/registry/skills/<name>/<version>.yaml
func LocalRegistryDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".quark", "registry")
}
