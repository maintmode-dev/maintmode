package apimodels

import (
	"time"

	"github.com/google/uuid"
)

// User is one entry in the admin users list.
//
// connected_providers and last_seen_at are placeholders until RUK-92 lands:
// connected_providers is always an empty (non-nil) array and last_seen_at is
// always null for now. oauth_provider is hardcoded to "google" (Google is the
// only provider), mirroring MeResponse.
type User struct {
	ID                 uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email              string     `json:"email"`
	DisplayName        string     `json:"display_name"`
	OAuthProvider      string     `json:"oauth_provider" example:"google"`
	ConnectedProviders []string   `json:"connected_providers"`
	Roles              []string   `json:"roles"`
	CreatedAt          time.Time  `json:"created_at" format:"date-time"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty" format:"date-time"`
	IsLastAdmin        bool       `json:"is_last_admin"`
	BlockedAt          *time.Time `json:"blocked_at,omitempty" format:"date-time"`
}

type ListUsersResponse struct {
	Users  []*User `json:"users"`
	Total  int64   `json:"total" example:"123"`
	Limit  int64   `json:"limit" example:"50"`
	Offset int64   `json:"offset" example:"0"`
}
