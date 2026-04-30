package store

import (
	"errors"
	"fmt"
)

// ErrAlreadyExists is returned by Create when a space with the same name exists.
var ErrAlreadyExists = errors.New("space already exists")

// NotFoundError is returned by Get and Delete when the requested id is absent.
type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("store: record %q not found", e.ID)
}

// NewNotFoundError constructs a NotFoundError for the given id.
func NewNotFoundError(id string) *NotFoundError {
	return &NotFoundError{ID: id}
}

// IsNotFound reports whether err is or wraps a *NotFoundError.
func IsNotFound(err error) bool {
	var nf *NotFoundError
	return errors.As(err, &nf)
}
