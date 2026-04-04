// Package space provides path helpers for the .quark/ directory structure.
package space

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	SubDir      = ".quark"
	DirSessions = "sessions"
	DirConfig   = "config"
	DirActivity = "activity"
	DirPlans    = "plans"
	DirKB       = "kb"
	DirPlugins  = "plugins"
)

func Path(spaceDir string) string {
	return filepath.Join(spaceDir, SubDir)
}

func PluginsDir(spaceDir string) string {
	return filepath.Join(Path(spaceDir), DirPlugins)
}

func KBDir(spaceDir string) string {
	return filepath.Join(Path(spaceDir), DirKB)
}

func ActivityDir(spaceDir string) string {
	return filepath.Join(Path(spaceDir), DirActivity)
}

func ActivityLogPath(spaceDir string) string {
	return filepath.Join(ActivityDir(spaceDir), "activity.jsonl")
}

func PlansDir(spaceDir string) string {
	return filepath.Join(Path(spaceDir), DirPlans)
}

func SessionsDir(spaceDir string) string {
	return filepath.Join(Path(spaceDir), DirSessions)
}

func ConfigDir(spaceDir string) string {
	return filepath.Join(Path(spaceDir), DirConfig)
}

// Exists checks if dir or any ancestor contains a Quarkfile.
func Exists(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, "Quarkfile")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// FindRoot walks up from dir and returns the directory that contains the
// Quarkfile. It returns an error if none is found.
func FindRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, "Quarkfile")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no Quarkfile found in %s or any ancestor", dir)
		}
		abs = parent
	}
}

// Resolve returns the absolute path of the space directory.
// It returns an error if no Quarkfile is found.
func Resolve(spaceDir string) (string, error) {
	abs, err := filepath.Abs(spaceDir)
	if err != nil {
		return "", err
	}
	if !Exists(abs) {
		return "", fmt.Errorf("no Quarkfile found in %s", abs)
	}
	return abs, nil
}

// Ensure creates the full .quark/ directory tree for a space.
func Ensure(spaceDir string) error {
	abs, err := filepath.Abs(spaceDir)
	if err != nil {
		return err
	}
	for _, d := range []string{
		SessionsDir(abs),
		ConfigDir(abs),
		ActivityDir(abs),
		PlansDir(abs),
		KBDir(abs),
		PluginsDir(abs),
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}
	return nil
}
