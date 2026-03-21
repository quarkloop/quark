package quarkfile

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// LockFile pins all external references to exact content hashes.
// Required for quark run — space will not start without it.
type LockFile struct {
	Quark      string        `yaml:"quark"`
	ResolvedAt *time.Time    `yaml:"resolved_at,omitempty"`
	Agents     []LockedAgent `yaml:"agents"`
	Tools      []LockedTool  `yaml:"tools"`
}

type LockedAgent struct {
	Ref      string `yaml:"ref"`
	Resolved string `yaml:"resolved"`
	Digest   string `yaml:"digest"`
}

type LockedTool struct {
	Ref      string `yaml:"ref"`
	Resolved string `yaml:"resolved"`
	Digest   string `yaml:"digest"`
}

func LoadLock(dir string) (*LockFile, error) {
	path := filepath.Join(dir, LockfileFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("lock file not found — run 'quark lock' first")
		}
		return nil, fmt.Errorf("reading lock file: %w", err)
	}
	var lf LockFile
	if err := yaml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing lock file: %w", err)
	}
	return &lf, nil
}

func SaveLock(dir string, lf *LockFile) error {
	lockDir := filepath.Join(dir, ".quark")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, LockfileFilename)
	data, err := yaml.Marshal(lf)
	if err != nil {
		return fmt.Errorf("marshaling lock file: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func LockExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, LockfileFilename))
	return err == nil
}
