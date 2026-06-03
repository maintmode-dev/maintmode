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
	ErrInvalidOAuthState    = errors.New("invalid oauth state")
	ErrOAuthStateTampered   = errors.New("oauth state tampered")
	ErrOAuthStateExpired    = errors.New("oauth state expired")
	ErrLogoutAlready        = errors.New("logout already")
	ErrUnsupportedProvider  = errors.New("unsupported provider")
	ErrAuthUnavailable      = errors.New("auth unavailable")
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
	ErrLastAdmin = fmt.Errorf("%w: cannot block or revoke admin from the last active admin", ErrValidation)
	ErrSelfBlock = fmt.Errorf("%w: cannot block yourself", ErrValidation)
)
