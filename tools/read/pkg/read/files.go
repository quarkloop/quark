package read

import (
	"fmt"
	"os"
	"strings"
)

func loadRegularTextFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s does not exist", path)
		}
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is a symlink, not a regular text file", path)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	if strings.ContainsRune(string(data), '\x00') {
		return "", fmt.Errorf("%s does not look like a text file", path)
	}

	return string(data), nil
}
