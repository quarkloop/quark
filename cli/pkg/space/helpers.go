package space

import "path/filepath"

func ensureAbs(dir string) (string, error) {
	if dir == "" {
		return ".", nil
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir), nil
	}
	return filepath.Abs(dir)
}
