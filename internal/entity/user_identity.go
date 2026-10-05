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
	IntegrationID uuid.UUID
	Subject       string
	Email         string
	CreatedAt     time.Time
}
