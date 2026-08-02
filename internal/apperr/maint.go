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

	// ErrApproverMismatch is returned when the user performing approve is neither
	// the one assigned as the maintenance approver nor an admin (admins may
	// approve any maintenance). Wraps ErrForbidden => HTTP 403.
	ErrApproverMismatch = fmt.Errorf("%w: only the assigned approver or an admin may approve this maintenance", ErrForbidden)

	// ErrNilActor guards approve against a missing actor. The approve decision
	// reads the actor's roles to grant admins the override, so a nil actor would
	// silently downgrade to the strict assignment check instead of failing. Like
	// ErrNilApprover, a nil actor cannot come from client input (it is resolved
	// from the access token), so it is an internal invariant violation and must
	// surface as HTTP 500.
	ErrNilActor = errors.New("actor must not be nil")

	// ErrNilApprover guards the personal approval queue against a zero approver
	// id. Without it the filter would silently collapse to "every draft in the
	// organization" instead of one user's queue. Deliberately NOT wrapping
	// ErrValidation/ErrForbidden: a zero id cannot come from client input (the
	// approver is taken from the access token), so this is an internal invariant
	// violation and must surface as HTTP 500, not 400.
	ErrNilApprover = errors.New("approver user id must not be nil")
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
