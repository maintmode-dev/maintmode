package apperr

import "errors"

var (
	ErrValidation             = errors.New("validation failed")
	ErrMethodNotAllowedInProd = errors.New("method not allowed in prod")
)
