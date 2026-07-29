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
	// TelegramTag and SlackTag are the user's messenger handles, shown exactly as
	// the user entered them, or null when not set. They are read-only on this
	// listing: users change their own handles via PATCH /me, and admins change
	// anyone's via PATCH /users/{id}. Both are matched by the list's search
	// parameter, so a handle from a complaint locates its owner.
	//
	// Surfacing them to admins is the feature's accountability mechanism, not
	// decoration. A handle is free text and unverified, so nothing stops someone
	// entering a colleague's handle and having maintenance notifications ping them;
	// this listing is what makes that attributable to a person.
	TelegramTag *string `json:"telegram_tag,omitempty"`
	SlackTag    *string `json:"slack_tag,omitempty"`
}

type ListUsersResponse struct {
	Users  []*User `json:"users"`
	Total  int64   `json:"total" example:"123"`
	Limit  int64   `json:"limit" example:"50"`
	Offset int64   `json:"offset" example:"0"`
}
