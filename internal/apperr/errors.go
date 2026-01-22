package apperr

import (
	"errors"
	"fmt"
)

var (
	ErrMaintNotFound                = errors.New("maint not found")
	ErrInvalidPeriodEmptyStartOrEnd = errors.New("invalid period: empty start or end")
	ErrInvalidPeriodStartOrEnd      = errors.New("invalid period: start > end or start == end")
	ErrConflictDetected             = errors.New("conflict detected")
	ErrForbiddenStatusTransition    = errors.New("forbidden status maintenance")
	ErrConflictsChangedSincePreview = errors.New("conflicts changed since preview")
	ErrMaintChangedSincePreview     = errors.New("maint changed since preview")
)

func ForbiddenStatusTransition(currentStatus any) error {
	return fmt.Errorf("%w: %v", ErrForbiddenStatusTransition, currentStatus)
}
