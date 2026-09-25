package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserIdentity links a user to a single sign-in method. A user may have several
// identities (one per method), enabling sign-in via any of them.
//
// The method is named by exactly one of two fields, and the database enforces
// that with a CHECK. IntegrationID points at the integration_settings row that
// vouches for the account; BuiltinMethod names a method that has no registry
// row at all -- break-glass, and the dev stub. Reading either without knowing
// which is set gives the wrong answer half the time, so ask Method().
type UserIdentity struct {
	ID     uuid.UUID
	UserID uuid.UUID
	// IntegrationID is the registry row this identity authenticates against,
	// or nil for a built-in method.
	IntegrationID *uuid.UUID
	// BuiltinMethod names the built-in method, or nil for a registry-backed
	// provider.
	BuiltinMethod *AuthMethod
	Subject       string
	Email         string
	CreatedAt     time.Time
}

// IsBuiltin reports whether the identity belongs to a method with no registry
// row. Its complement is "registry-backed": the CHECK admits no third state.
func (i *UserIdentity) IsBuiltin() bool {
	return i.BuiltinMethod != nil
}

// SignInMethodRef names the sign-in method a query or a write is about.
//
// A method reaches user_identities through one of two columns: a registry-backed
// provider by the id of its integration_settings row, a built-in method by name.
// Which one is a fact about the METHOD, so it is decided once, where the method
// is known, rather than at each of the four places that touch those columns.
//
// Build it with SignInByIntegration or SignInByBuiltin. The zero value names
// nothing and every consumer refuses it, so a caller that forgets to set one
// gets a loud failure rather than a query that matches every row.
type SignInMethodRef struct {
	// IntegrationID is the registry row, set for a configured provider.
	IntegrationID *uuid.UUID
	// BuiltinMethod is the method's own name, set when it has no registry row.
	BuiltinMethod *AuthMethod
}

// SignInByIntegration names a provider configured in the registry.
func SignInByIntegration(id uuid.UUID) SignInMethodRef {
	return SignInMethodRef{IntegrationID: &id}
}

// SignInByBuiltin names a method that authenticates without a registry row.
func SignInByBuiltin(method AuthMethod) SignInMethodRef {
	return SignInMethodRef{BuiltinMethod: &method}
}

// IsZero reports that the reference names no method at all.
func (r SignInMethodRef) IsZero() bool {
	return r.IntegrationID == nil && r.BuiltinMethod == nil
}

// Apply writes the reference onto an identity about to be stored.
//
// Both columns are assigned together -- one set, the other cleared -- because
// assigning only the intended one leaves whatever the struct carried before.
// That mistake compiles and is caught by a CHECK at write time, which is late.
func (r SignInMethodRef) Apply(identity *UserIdentity) {
	identity.IntegrationID = r.IntegrationID
	identity.BuiltinMethod = r.BuiltinMethod
}
