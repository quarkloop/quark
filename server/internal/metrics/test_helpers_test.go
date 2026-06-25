// Package metrics — test helpers (test logger).
package metrics

import "go.uber.org/zap"

// testLogger returns a no-op zap logger suitable for tests.
func testLogger() *zap.Logger {
	return zap.NewNop()
}
