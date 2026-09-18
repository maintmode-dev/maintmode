package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuthMethodSetting is one built-in sign-in method's on/off flag.
//
// "Built-in" is the whole scope: email_otp and email_password, the two methods
// this deployment implements itself. Login PROVIDERS are configured through the
// integration registry and carry their own enabled flag there, so they are
// deliberately absent from this type -- two switches over one provider would
// mean every read had to answer which of them wins.
//
// bootstrap is absent too, and for a different reason: it is the break-glass
// credential, always enabled and never listed, and giving it a row would create
// a switch that must never be thrown.
type AuthMethodSetting struct {
	ID      uuid.UUID
	Method  AuthMethodName
	Enabled bool
	// UpdatedByUserID is the admin who last flipped it, captured from the access
	// token with no FK to users. It is stored but not resolved to a name on
	// read: the audit trail already answers "who changed this", and a second
	// answer here would be the same question asked twice.
	UpdatedByUserID *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AuthMethodName is the closed set of built-in methods that can be toggled.
//
// The values are the same strings GET /auth/providers puts on the wire, and
// that identity is deliberate: the listing renders them as literals, so the two
// spellings are independent and a test crossing them proves the contract rather
// than comparing a constant against itself.
type AuthMethodName string

const (
	// AuthMethodNameEmailOTP is sign-in by a code mailed to the address.
	AuthMethodNameEmailOTP AuthMethodName = "email_otp"
	// AuthMethodNameEmailPassword is sign-in with the user's own password.
	//
	// Disabling it does NOT disable break-glass, which answers on the same
	// route: see services/auth/login_with_password.go, where the gate guards
	// entry to the stored-password step rather than the request.
	AuthMethodNameEmailPassword AuthMethodName = "email_password"
)

// IsValid reports whether the name is one this deployment can toggle.
//
// A closed set rather than a CHECK constraint, matching integration_settings:
// which methods exist is a fact about the code, and a name the code cannot
// enforce would be a row nothing reads.
func (m AuthMethodName) IsValid() bool {
	switch m {
	case AuthMethodNameEmailOTP, AuthMethodNameEmailPassword:
		return true
	default:
		return false
	}
}

// AllAuthMethodNames returns the closed set, in the order a listing renders it.
//
// Derived from nothing: it IS the set. Callers that need "every built-in"
// -- the seeded-completeness check, the admin listing -- read it here rather
// than writing the two names again.
func AllAuthMethodNames() []AuthMethodName {
	return []AuthMethodName{AuthMethodNameEmailOTP, AuthMethodNameEmailPassword}
}

// SetAuthMethodEnabledCmd asks for one method's flag to be set to a value.
//
// SetEnabled rather than Toggle: the caller states the target state, so a
// retried request converges instead of flipping back.
type SetAuthMethodEnabledCmd struct {
	Method  AuthMethodName
	Enabled bool
	Actor   *User
}
