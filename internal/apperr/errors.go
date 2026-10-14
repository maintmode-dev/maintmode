package apperr

import "errors"

var (
	ErrValidation = errors.New("validation failed")
	ErrNotFound   = errors.New("not found")
)
