package model

import (
	"net"
	"strings"
)

// isRetryableError returns true for transient errors that warrant trying
// the next gateway in the fallback chain.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Network-level timeouts are always retryable.
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	msg := err.Error()
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") {
		return true
	}
	if strings.Contains(msg, "500") || strings.Contains(msg, "502") || strings.Contains(msg, "503") {
		return true
	}
	if strings.Contains(msg, "connection refused") {
		return true
	}
	return false
}
