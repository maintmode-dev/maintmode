package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserIdentity links a user to a single sign-in method. A user may have several
// identities (one per method), enabling sign-in via any of them.
type UserIdentity struct {
	ID     uuid.UUID
	UserID uuid.UUID
	// IntegrationID is the registry row this identity authenticates against.
	IntegrationID *uuid.UUID
	Subject       string
	Email         string
	CreatedAt     time.Time
}

// SignInMethodRef names the sign-in method a query or a write is about: a
// provider, by the id of its integration_settings row.
//
// Build it with SignInByIntegration. The zero value names nothing and every
// consumer refuses it, so a caller that forgets to set one gets a loud failure
// rather than a query that matches every row.
type SignInMethodRef struct {
	// IntegrationID is the registry row.
	IntegrationID *uuid.UUID
}

// SignInByIntegration names a provider configured in the registry.
func SignInByIntegration(id uuid.UUID) SignInMethodRef {
	return SignInMethodRef{IntegrationID: &id}
}

// IsZero reports that the reference names no method at all.
func (r SignInMethodRef) IsZero() bool {
	return r.IntegrationID == nil
}

// Apply writes the reference onto an identity about to be stored.
func (r SignInMethodRef) Apply(identity *UserIdentity) {
	identity.IntegrationID = r.IntegrationID
}
