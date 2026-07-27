package apperr

import (
	"errors"
	"fmt"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrLockBusy             = errors.New("lock is already held")
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrInvalidAccessToken   = errors.New("invalid access token")
	ErrInvalidRefreshToken  = errors.New("invalid refresh token")
	ErrTokenExpired         = errors.New("token expired")
	ErrTokenReuse           = errors.New("token reuse detected")
	ErrSuspiciousActivity   = errors.New("suspicious activity detected")
	ErrLogoutAlready        = errors.New("logout already")
	ErrUnsupportedProvider  = errors.New("unsupported provider")
	ErrAuthUnavailable      = errors.New("auth unavailable")
	// ErrUserBlocked marks a blocked user trying to obtain or use an access
	// token. Issuance (login/refresh/re-issue) and introspection both reject it,
	// so blocking a user cuts off both new tokens and live ones on the next
	// introspected (critical-mutation) request.
	ErrUserBlocked = errors.New("user is blocked")
	// ErrSignupDisabled rejects an OAuth login of an unknown user when neither an
	// invitation nor open signup authorizes creating the account. No user row is
	// persisted. 403 (not 401): the OIDC identity is verified — account creation
	// is what is refused. The HTTP message stays generic so the response never
	// reveals whether an invitation exists for the email.
	ErrSignupDisabled = errors.New("signup disabled")
)

// OAuth provider connect/disconnect errors.
var (
	ErrProviderAlreadyConnected    = errors.New("provider already connected")
	ErrProviderLinkedToAnotherUser = errors.New("provider linked to another user")
	// ErrProviderNotConnected is an internal "identity row not found" marker used
	// by identity lookups; it is handled inside the service layer (never mapped
	// to an HTTP status), so disconnect stays idempotent.
	ErrProviderNotConnected         = errors.New("provider not connected")
	ErrCannotDisconnectLastProvider = errors.New("cannot disconnect the only sign-in method")
)

var (
	ErrInvalidRole = errors.New("invalid role")
	ErrForbidden   = errors.New("forbidden")
	ErrNotChanged  = errors.New("not changed")
)

// User management lockout protection. Both wrap ErrValidation so the HTTP layer
// maps them to 400 (see httperrors.ToAPIError).
var (
	ErrLastAdmin  = fmt.Errorf("%w: cannot block or revoke admin from the last active admin", ErrValidation)
	ErrSelfBlock  = fmt.Errorf("%w: cannot block yourself", ErrValidation)
	ErrSelfRevoke = fmt.Errorf("%w: cannot revoke a role from yourself", ErrValidation)
	// ErrInvalidTimezone is returned when a timezone preference is not a valid
	// IANA identifier (checked via time.LoadLocation). Wraps ErrValidation → 400.
	ErrInvalidTimezone = fmt.Errorf("%w: invalid timezone", ErrValidation)
	// ErrInvalidMessengerTag is returned when a messenger handle fails the
	// charset/length allowlist, or is a Slack broadcast value (here / channel /
	// everyone). Wraps ErrValidation → 400.
	ErrInvalidMessengerTag = fmt.Errorf("%w: invalid messenger tag", ErrValidation)
)

// User invitation errors.
//
// The accept endpoint must never leak which precondition failed: a holder of a
// token-link must not learn the invited email, the roles, or even whether the
// token maps to a real invitation. The public-facing failures therefore wrap
// ErrValidation → HTTP 400, and the accept handler translates them into a bare
// {status: "invalid"|"email_mismatch"} body with no further detail.
var (
	ErrInvitationNotFound   = errors.New("invitation not found")
	ErrInvitationNotPending = errors.New("invitation is not pending")
	ErrInvitationExpired    = errors.New("invitation expired")
	ErrUserAlreadyExists    = errors.New("user already exists")
	ErrActivePendingExists  = errors.New("active pending invitation already exists")
	// ErrInvalidInvitation covers any accept-time failure that must surface as a
	// generic "invalid" status: token not found, expired, already accepted,
	// revoked, or an unverifiable OAuth token.
	ErrInvalidInvitation = fmt.Errorf("%w: invitation invalid", ErrValidation)
	// ErrEmailMismatch is the accept-time guard: the OAuth email does not match
	// the invitation email. Surfaced as status "email_mismatch" with no detail.
	ErrEmailMismatch = fmt.Errorf("%w: email mismatch", ErrValidation)
)
