package apperr

import (
	"errors"
	"fmt"
)

// #######################################
// ################ maint ################
// #######################################
var (
	ErrMaintNotFound                = errors.New("maintenance not found")
	ErrInvalidPeriodEmptyStartOrEnd = errors.New("invalid period: empty start or end")
	ErrConflictsChangedSincePreview = errors.New("conflicts changed since preview")
	ErrMaintChangedSincePreview     = errors.New("maintenance changed since preview")

	ErrInvalidPeriodStartOrEnd        = fmt.Errorf("%w: invalid period: start > end or start == end", ErrValidation)
	ErrInvalidPeriodInterval          = fmt.Errorf("%w: invalid period interval", ErrValidation)
	ErrForbiddenMaintStatusTransition = fmt.Errorf("%w: forbidden maint status", ErrValidation)
)

func ForbiddenMaintStatusTransition(currentStatus, newStatus any) error {
	return fmt.Errorf("%w: from %v to %v", ErrForbiddenMaintStatusTransition, currentStatus, newStatus)
}

// ###########################################
// ################ resources ################
// ###########################################
var (
	ErrResourceNotFound      = errors.New("resource not found")
	ErrResourceAlreadyExists = errors.New("resource already exists")
)

// #######################################
// ################ steps ################
// #######################################
var (
	ErrStepNotFound                  = errors.New("maintenance step not found")
	ErrStepOrderViolation            = errors.New("maintenance step order violation")
	ErrMaintenanceHasUnfinishedSteps = errors.New("maintenance has unfinished steps")

	ErrForbiddenStepStatusTransition = fmt.Errorf("%w: forbidden step maint status", ErrValidation)
)

func ForbiddenStepStatusTransition(currentStatus, newStatus any) error {
	return fmt.Errorf("%w: from %v to %v", ErrForbiddenStepStatusTransition, currentStatus, newStatus)
}

// ###############################################
// ########## notify targets #####################
// ###############################################
var (
	ErrNotifyTargetsAlreadyExists = errors.New("notify targets already exists")
)

// ###############################################
// ########## notify channels ####################
// ###############################################
var (
	ErrNotifyChannelAlreadyExists = errors.New("notify channel already exists")
)
