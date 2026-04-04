package space

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quarkloop/cli/pkg/quarkfile"
)

func runValidate(dir string) error {
	absDir, err := ensureAbs(dir)
	if err != nil {
		return err
	}

	qf, err := quarkfile.Load(absDir)
	if err != nil {
		return fmt.Errorf("quarkfile invalid: %w", err)
	}
	if err := quarkfile.Validate(absDir, qf); err != nil {
		return fmt.Errorf("quarkfile validation failed: %w", err)
	}

	// Check that .quark/plugins/ directory exists and each plugin has a valid manifest.
	plugDir := filepath.Join(absDir, ".quark", "plugins")
	if stat, err := os.Stat(plugDir); err != nil || !stat.IsDir() {
		return fmt.Errorf(".quark/plugins/ directory missing — run 'quark plugin install'")
	}

	entries, err := os.ReadDir(plugDir)
	if err != nil {
		return fmt.Errorf("reading .quark/plugins/: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(plugDir, e.Name(), "manifest.yaml")
		if _, err := os.Stat(manifestPath); err != nil {
			return fmt.Errorf("plugin %q: manifest.yaml missing — %w", e.Name(), err)
		}
	}

	// Verify all plugins referenced in Quarkfile are installed on disk.
	installed := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			installed[e.Name()] = true
		}
	}
	for _, pref := range qf.Plugins {
		name := extractPluginName(pref.Ref)
		if !installed[name] {
			return fmt.Errorf("plugin %q (ref: %q) referenced in Quarkfile but not installed", name, pref.Ref)
		}
	}

	return nil
}

// extractPluginName extracts the plugin directory name from a ref.
//   - "quark/tool-bash" → "tool-bash"
//   - "github.com/user/tool-jupyter" → "tool-jupyter"
func extractPluginName(ref string) string {
	if idx := strings.LastIndexByte(ref, '/'); idx >= 0 {
		return ref[idx+1:]
	}
	return ref
}
