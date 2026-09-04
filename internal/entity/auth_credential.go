package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuthCredentialKind distinguishes the two secrets held in auth_credentials.
// It participates in the partial unique indexes and in every query predicate,
// which is what keeps a password row off a one-time-code read path.
type AuthCredentialKind string

const (
	AuthCredentialKindPassword AuthCredentialKind = "password"
	AuthCredentialKindOTP      AuthCredentialKind = "otp"
)

// AuthCredential is a user secret held as a hash. SecretHash carries a sha256
// digest for a one-time code and an argon2id PHC string for a password; the
// format is self-describing, so a verifier reads the algorithm out of the value
// rather than assuming it from context.
//
// Compare the sha256 form with crypto/subtle.ConstantTimeCompare, never with
// ==: a byte-wise comparison on a hex digest returns early on the first
// mismatch and leaks the shared prefix length through response timing. The
// argon2id form is compared by a function that is already constant-time. See
// services/authmethod/bootstrapauth/authenticate.go for the precedent.
//
// ExpiresAt, ConsumedAt and SessionNonce are meaningful only for a one-time
// code and are nil for a password. Nothing at this layer enforces that: which
// combinations are legal is the calling service's policy.
type AuthCredential struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Kind         AuthCredentialKind
	SecretHash   string
	ExpiresAt    *time.Time
	ConsumedAt   *time.Time
	Attempts     int16
	SessionNonce *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
