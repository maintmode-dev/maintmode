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

	// ErrConcurrentModification means a serializable transaction kept losing to
	// concurrent writers and exhausted its retries. It is safe (and expected) for
	// the client to retry the whole operation. Mapped to HTTP 409.
	ErrConcurrentModification = errors.New("concurrent modification, please retry")

	ErrInvalidPeriodStartOrEnd        = fmt.Errorf("%w: invalid period: start > end or start == end", ErrValidation)
	ErrInvalidPeriodInterval          = fmt.Errorf("%w: invalid period interval", ErrValidation)
	ErrForbiddenMaintStatusTransition = fmt.Errorf("%w: forbidden maint status", ErrValidation)

	// ErrApproverNotEligible is returned when the user assigned as a maintenance
	// approver does not exist, is blocked, or lacks the reviewer/admin role.
	// Wraps ErrValidation => HTTP 400.
	ErrApproverNotEligible = fmt.Errorf("%w: approver is not an active reviewer/admin", ErrValidation)

	// ErrApproverMismatch is returned when the user performing approve is not the
	// one assigned as the maintenance approver. Wraps ErrForbidden => HTTP 403.
	ErrApproverMismatch = fmt.Errorf("%w: only the assigned approver may approve this maintenance", ErrForbidden)
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
	ErrNotifyChannelNotFound      = errors.New("notify channel not found")
)
