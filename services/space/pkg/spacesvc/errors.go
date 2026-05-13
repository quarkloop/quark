package spacesvc

import "errors"

var ErrAlreadyExists = errors.New("space already exists")

type NotFoundError struct {
	Name string
}

func (e NotFoundError) Error() string { return "space not found: " + e.Name }

func IsNotFound(err error) bool {
	var notFound NotFoundError
	return errors.As(err, &notFound)
}
