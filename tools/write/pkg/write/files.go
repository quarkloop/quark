package write

import (
	"fmt"
	"os"
	"strings"
)

type resultStats struct {
	Created      bool
	Changed      bool
	BytesWritten int
	Replacements int
	EditsApplied int
}

func statRegularTextFile(path string) (os.FileInfo, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("%s is a symlink, not a regular text file", path)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%s is not a regular file", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("reading %s: %w", path, err)
	}
	if strings.ContainsRune(string(data), '\x00') {
		return nil, false, fmt.Errorf("%s does not look like a text file", path)
	}

	return info, true, nil
}

func writeFile(path string, content string, mode os.FileMode) error {
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

func fileMode(info os.FileInfo, exists bool) os.FileMode {
	if exists && info != nil {
		return info.Mode().Perm()
	}
	return 0644
}

func buildResult(path, operation string, stats resultStats) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("reading result file: %w", err)
	}

	return Result{
		Path:           path,
		Operation:      operation,
		Created:        stats.Created,
		Changed:        stats.Changed,
		BytesWritten:   stats.BytesWritten,
		FileSize:       len(data),
		Replacements:   stats.Replacements,
		EditsApplied:   stats.EditsApplied,
		ContentPreview: truncatePreview(string(data), contentPreviewLimit),
	}, nil
}

func truncatePreview(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
