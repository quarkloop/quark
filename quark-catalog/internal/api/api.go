// Package api defines the JSON request and response types exchanged
// between the Java platform and the Go Catalog service over NATS
// request-reply.
//
// Each subject (e.g. "catalog.system.get") carries a JSON body whose
// shape is one of the structs in this package. Types are grouped by
// domain into separate files (systems.go, nodes.go, ...) but all live
// in package api so callers can import them with a single alias.
//
// Conventions:
//   - Request structs end in "Request".
//   - Response structs end in "Response".
//   - List responses end in "ListResponse" and contain a slice field
//     named after the entity (e.g. Systems []SystemResponse).
//   - All timestamps are RFC3339Nano strings (SQLite stores them as TEXT).
package api

import "fmt"

// ErrorResponse is the generic envelope used for write operations and
// as the error body for any failed request. The Success field lets the
// Java side distinguish success from failure without inspecting the
// Error string.
type ErrorResponse struct {
	Error   string `json:"error,omitempty"`
	Success bool   `json:"success"`
}

// NewError constructs an ErrorResponse with a formatted message.
func NewError(format string, args ...any) ErrorResponse {
	return ErrorResponse{Error: fmt.Sprintf(format, args...), Success: false}
}

// OK is the canonical success response. Callers should never mutate it.
var OK = ErrorResponse{Success: true}

// NotFound is a convenience response for "not found" errors.
var NotFound = ErrorResponse{Success: false, Error: "not found"}
