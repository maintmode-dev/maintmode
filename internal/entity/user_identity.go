package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserIdentity links a user to a single OAuth provider account. A user may have
// several identities (one per provider), enabling sign-in via any of them.
type UserIdentity struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Provider  OAuthProvider
	Subject   string
	Email     string
	CreatedAt time.Time
}
