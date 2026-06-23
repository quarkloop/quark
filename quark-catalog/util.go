// Package main — utility functions.
package main

import (
	"crypto/sha256"
	"encoding/hex"
)

// sha256hex returns the hex-encoded SHA-256 hash of data.
func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
