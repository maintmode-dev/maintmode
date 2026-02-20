package apperr

import (
	"errors"
	"fmt"
)

var (
	ErrMaintNotFound                = errors.New("maintenance not found")
	ErrResourceNotFound             = errors.New("resource not found")
	ErrInvalidPeriodEmptyStartOrEnd = errors.New("invalid period: empty start or end")
	ErrConflictsChangedSincePreview = errors.New("conflicts changed since preview")
	ErrMaintChangedSincePreview     = errors.New("maintenance changed since preview")
)

var (
	ErrValidation                = errors.New("validation failed")
	ErrInvalidPeriodStartOrEnd   = fmt.Errorf("%w: invalid period: start > end or start == end", ErrValidation)
	ErrInvalidPeriodInterval     = fmt.Errorf("%w: invalid period interval", ErrValidation)
	ErrForbiddenStatusTransition = fmt.Errorf("%w: forbidden status maintenance", ErrValidation)
)

func ForbiddenStatusTransition(currentStatus any) error {
	return fmt.Errorf("%w: %v", ErrForbiddenStatusTransition, currentStatus)
}
