package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// LocalService implements Service using the filesystem and existing
// plugin Manager, HubClient, and installer functions.
type LocalService struct {
	manager *Manager
	hub     *HubClient
}

// NewLocalService creates a plugin service.
func NewLocalService() Service {
	hubURL := os.Getenv("QUARK_PLUGIN_HUB_URL")
	return &LocalService{
		manager: NewManager(),
		hub:     NewHubClient(hubURL),
	}
}

func (s *LocalService) Install(_ context.Context, ref, pluginsDir string) (*Manifest, error) {
	return InstallPlugin(pluginsDir, ref)
}

func (s *LocalService) Uninstall(_ context.Context, name, pluginsDir string) error {
	return s.manager.Uninstall(pluginsDir, name)
}

func (s *LocalService) List(_ context.Context, pluginsDir string) ([]Plugin, error) {
	return s.manager.List(pluginsDir)
}

func (s *LocalService) Info(_ context.Context, name, pluginsDir string) (*Plugin, error) {
	return s.manager.Get(pluginsDir, name)
}

func (s *LocalService) Search(_ context.Context, query string) ([]PluginSearchItem, error) {
	return s.hub.Search(query)
}

func (s *LocalService) Build(_ context.Context, dir string) (*Manifest, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve dir: %w", err)
	}
	man, err := LoadLocal(absDir)
	if err != nil {
		return nil, err
	}
	if man.Type == TypeTool {
		if err := buildToolBinary(absDir); err != nil {
			return nil, fmt.Errorf("build tool binary: %w", err)
		}
	}
	return man, nil
}

func (s *LocalService) Update(_ context.Context, name, pluginsDir string) (*Manifest, error) {
	return s.manager.Update(pluginsDir, "plugins", name)
}

// buildToolBinary compiles a Go tool plugin in the given directory.
func buildToolBinary(dir string) error {
	entries, err := os.ReadDir(filepath.Join(dir, "cmd"))
	if err != nil {
		return fmt.Errorf("read cmd/ dir: %w", err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cmdPath := filepath.Join(dir, "cmd", e.Name())
		outPath := filepath.Join(binDir, e.Name())
		cmd := exec.Command("go", "build", "-o", outPath, "./"+cmdPath)
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("build %s: %w", e.Name(), err)
		}
	}
	return nil
}
