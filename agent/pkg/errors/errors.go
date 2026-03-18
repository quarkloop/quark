// Package errors defines structured error types for the Quark system.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common failure conditions.
var (
	ErrNotFound            = errors.New("not found")
	ErrAlreadyExists       = errors.New("already exists")
	ErrInvalidInput        = errors.New("invalid input")
	ErrNoLockFile          = errors.New("lock file not found — run 'quark lock' first")
	ErrNoQuarkfile         = errors.New("Quarkfile not found in directory")
	ErrSpaceNotFound       = errors.New("space not found")
	ErrSpaceRunning        = errors.New("space is already running")
	ErrRegistryUnreachable = errors.New("registry unreachable")
	ErrNoAvailablePorts    = errors.New("no available ports")
	ErrProcessNotFound     = errors.New("space has no live process")
)

// QuarkError is a structured error with an HTTP-friendly status code.
type QuarkError struct {
	Code    int
	Message string
	Err     error
}

func (e *QuarkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *QuarkError) Unwrap() error { return e.Err }

// New creates a QuarkError with the given code, message, and optional wrapped error.
func New(code int, message string, err error) *QuarkError {
	return &QuarkError{Code: code, Message: message, Err: err}
}

// NotFound creates a 404 QuarkError.
func NotFound(msg string) *QuarkError {
	return &QuarkError{Code: 404, Message: msg, Err: ErrNotFound}
}

// BadRequest creates a 400 QuarkError.
func BadRequest(msg string) *QuarkError {
	return &QuarkError{Code: 400, Message: msg, Err: ErrInvalidInput}
}

// Internal creates a 500 QuarkError.
func Internal(msg string, err error) *QuarkError {
	return &QuarkError{Code: 500, Message: msg, Err: err}
}
