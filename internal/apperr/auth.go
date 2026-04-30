package apperr

import "errors"

var (
	ErrUserNotFound             = errors.New("user not found")
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
