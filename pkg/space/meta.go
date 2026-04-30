package space

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Metadata is the persisted metadata for a space directory.
type Metadata struct {
	Name       string    `json:"name"`
	WorkingDir string    `json:"working_dir"`
	Version    string    `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ReadMetadata loads metadata from path. Missing files return os.ErrNotExist.
func ReadMetadata(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("read meta: %w", err)
	}
	var sp Metadata
	if err := json.Unmarshal(data, &sp); err != nil {
		return nil, fmt.Errorf("parse meta: %w", err)
	}
	return &sp, nil
}

// WriteMetadata atomically stores metadata at path.
func WriteMetadata(path string, sp *Metadata) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create meta dir: %w", err)
	}
	data, err := json.MarshalIndent(sp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename meta: %w", err)
	}
	return nil
}
