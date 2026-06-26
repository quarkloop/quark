// Package model contains plain Go structs that map 1:1 to the server's
// REST API JSON responses. They are intentionally dumb data holders — no
// methods, no logic. The tags use the same JSON field names as the server
// DTOs so encoding/json can marshal/unmarshal them directly.
package model

// ErrorResponse is the standard error shape returned by all endpoints.
type ErrorResponse struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ValidationErrorDto is one item in a deploy-failure response.
type ValidationError struct {
	Path     string `json:"path"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}
