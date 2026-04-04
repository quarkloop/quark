// Package middleware provides PersistentPreRunE hooks for cobra commands.
package middleware

import (
	"fmt"
	"os"

	"github.com/quarkloop/core/pkg/space"
)

// RequireSpace returns an error if the current working directory
// (or any ancestor) does not contain a Quarkfile.
func RequireSpace() error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if !space.Exists(dir) {
		return fmt.Errorf("not in a quark space — no Quarkfile found in %s or any ancestor directory", dir)
	}
	return nil
}
