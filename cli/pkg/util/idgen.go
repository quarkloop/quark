// Package util provides utility functions for the CLI.
package util

import (
	"crypto/rand"
	"encoding/hex"
)

// ShortID generates a short random ID (8 hex characters).
func ShortID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
