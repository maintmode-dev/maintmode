package apimodels

import (
	"time"

	"github.com/google/uuid"
)

// User is one entry in the admin users list.
//
// connected_providers lists every OAuth provider linked to the user (from
// user_identities) and is always a non-nil array. oauth_provider mirrors
// MeResponse and reports the first linked provider, or "unknown" when none are
// linked. last_seen_at is null for now (not tracked yet).
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

// S2SUser is the neutral, lightweight user representation returned by the
// service-to-service users listing. It carries only identity and roles — no
// admin-management fields (is_last_admin, blocked_at, providers) — so consuming
// services (e.g. maintmode assignment pickers) get just what selection needs.
type S2SUser struct {
	ID          uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Roles       []string  `json:"roles"`
}

type ListS2SUsersResponse struct {
	Users  []*S2SUser `json:"users"`
	Total  int64      `json:"total" example:"123"`
	Limit  int64      `json:"limit" example:"50"`
	Offset int64      `json:"offset" example:"0"`
}
