package store

import "fmt"

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

// IsNotFound reports whether err is a *NotFoundError.
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}
