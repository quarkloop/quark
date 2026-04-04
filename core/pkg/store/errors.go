package store

import (
	"errors"
	"fmt"
)

// NotFoundError is returned by Get and Delete when the requested id is absent.
type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("store: record %q not found", e.ID)
}

// ErrNotFound constructs a NotFoundError for the given id.
func ErrNotFound(id string) *NotFoundError {
	return &NotFoundError{ID: id}
}

// IsNotFound reports whether err is or wraps a *NotFoundError.
func IsNotFound(err error) bool {
	var nf *NotFoundError
	return errors.As(err, &nf)
}
