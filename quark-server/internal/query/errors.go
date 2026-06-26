// Package query — sentinel errors + tiny helpers used across the
// query services.
package query

import (
	"errors"
	"runtime"
)

// ErrNotFound is returned by GetXxx methods when no matching record
// exists in the Catalog. The HTTP handler converts this to a 404.
var ErrNotFound = errors.New("not found")

// availableProcessors returns runtime.NumCPU(). Centralised here so
// tests can stub it.
func availableProcessors() int {
	return runtime.NumCPU()
}
