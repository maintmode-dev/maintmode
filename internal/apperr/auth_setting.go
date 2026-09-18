package apperr

import "errors"

// ErrAuthMethodNotFound is returned for a method name outside the closed set,
// and for a known name whose row is missing.
//
// The second case is a fault, not a default: the migration seeds every
// built-in, so a missing row means something lost it. Guessing "enabled" would
// silently reopen a sign-in path an admin closed; guessing "disabled" would
// take one away. A loud error is recoverable where either guess is not.
var ErrAuthMethodNotFound = errors.New("auth method not found")
