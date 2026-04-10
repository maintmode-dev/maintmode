package apperr

import (
	"errors"
	"fmt"
)

var (
	ErrMaintNotFound    = errors.New("maintenance not found")
	ErrResourceNotFound = errors.New("resource not found")
	ErrUserNotFound     = errors.New("user not found")

	ErrResourceAlreadyExists        = errors.New("resource already exists")
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

var (
	ErrLockBusy                 = errors.New("lock is already held")
	ErrRefreshTokenNotFound     = errors.New("refresh token not found")
	ErrInvalidAccessTokenToken  = errors.New("invalid access token")
	ErrInvalidRefreshTokenToken = errors.New("invalid refresh token")
	ErrTokenExpired             = errors.New("token expired")
	ErrTokenReuse               = errors.New("token reuse detected")
	ErrSuspiciousActivity       = errors.New("suspicious activity detected")
	ErrInvalidOAuthState        = errors.New("invalid oauth state")
	ErrLogoutAlready            = errors.New("logout already")
	ErrUnsupportedProvider      = errors.New("unsupported provider")
)

var (
	ErrInvalidRole = errors.New("invalid role")
	ErrNotChanged  = errors.New("not changed")
)
