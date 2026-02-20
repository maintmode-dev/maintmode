package apierrors

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
)

var (
	ErrInvalidUUID = errValidation("id must be a valid UUID")
	ErrParseBody   = errValidation("parse request body failed")
)

func ValidationErr(err error) error {
	return errValidation(err.Error())
}

func errValidation(err string) error {
	return fmt.Errorf("%w: %s", apperr.ErrValidation, err)
}
