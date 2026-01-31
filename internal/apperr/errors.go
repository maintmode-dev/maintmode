package apperr

import (
	"errors"
	"fmt"
)

var (
	ErrMaintNotFound                = errors.New("maintenance not found")
	ErrResourceNotFound             = errors.New("resource not found")
	ErrInvalidPeriodEmptyStartOrEnd = errors.New("invalid period: empty start or end")
	ErrInvalidPeriodStartOrEnd      = errors.New("invalid period: start > end or start == end")
	ErrInvalidPeriodInterval        = errors.New("invalid period interval")
	ErrForbiddenStatusTransition    = errors.New("forbidden status maintenance")
	ErrConflictsChangedSincePreview = errors.New("conflicts changed since preview")
	ErrMaintChangedSincePreview     = errors.New("maintenance changed since preview")
)

func ForbiddenStatusTransition(currentStatus any) error {
	return fmt.Errorf("%w: %v", ErrForbiddenStatusTransition, currentStatus)
}
