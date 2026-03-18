package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runAddKB(dir, path string, value []byte) error {
	dest := filepath.Join(dir, "kb", filepath.FromSlash(path))
	if filepath.Ext(dest) == "" {
		dest += ".yaml"
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("creating kb dir: %w", err)
	}
	return os.WriteFile(dest, value, 0644)
}

func runRemoveKB(dir, path string) error {
	dest := filepath.Join(dir, "kb", filepath.FromSlash(path))
	return os.Remove(dest)
}

func runListKB(dir string) ([]KBEntry, error) {
	kbDir := filepath.Join(dir, "kb")
	var entries []KBEntry
	err := filepath.Walk(kbDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(kbDir, path)
		entries = append(entries, KBEntry{
			Path: strings.ReplaceAll(rel, string(filepath.Separator), "/"),
			Size: int(info.Size()),
		})
		return nil
	})
	return entries, err
}

func runShowKB(dir, path string) ([]byte, error) {
	dest := filepath.Join(dir, "kb", filepath.FromSlash(path))
	return os.ReadFile(dest)
}
