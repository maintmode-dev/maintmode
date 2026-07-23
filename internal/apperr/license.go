package apperr

import "errors"

// License enforcement errors. Both map to HTTP 403 with stable,
// machine-readable codes — the frontend contract (see httperrors).
var (
	// ErrSeatsLimitExceeded rejects inviting or activating a user when occupied
	// seats (active seat-users + live pending seat-invites) have reached the
	// licensed seats_purchased.
	ErrSeatsLimitExceeded = errors.New("seats limit exceeded")
	// ErrOrganizationSuspended is returned by the license suspend middleware for
	// every application request while the license is suspended, canceled, or the
	// trial has expired. Data is preserved; access is blocked.
	ErrOrganizationSuspended = errors.New("organization suspended")
	// ErrLicenseCacheEmpty marks a license-enabled instance that has no cached
	// license yet (the first heartbeat has not succeeded). Handled inside the
	// license service — never mapped to an HTTP status.
	ErrLicenseCacheEmpty = errors.New("license cache is empty")
)
