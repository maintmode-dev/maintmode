package apimodels

import (
	"time"

	"github.com/google/uuid"
)

// CreateInvitationRequest is the admin request body for POST /users/invite.
type CreateInvitationRequest struct {
	Email string   `json:"email" example:"new.user@example.com"`
	Roles []string `json:"roles" example:"editor"`
}

// CreateInvitationResponse is returned on 201 from POST /users/invite.
type CreateInvitationResponse struct {
	InvitationID uuid.UUID `json:"invitation_id"`
	Email        string    `json:"email"`
	Roles        []string  `json:"roles"`
	ExpiresAt    time.Time `json:"expires_at" format:"date-time"`
	Status       string    `json:"status" example:"pending"`
}

// InvitedBy is the minimal inviter reference shown in the admin list.
type InvitedBy struct {
	ID     uuid.UUID `json:"id"`
	Handle string    `json:"handle" example:"system"`
}

// Invitation is the admin-view shape (GET /users/invitations). It MAY contain
// sensitive fields (email, roles, inviter) — this endpoint is admin-only.
type Invitation struct {
	ID         uuid.UUID  `json:"id"`
	Email      string     `json:"email"`
	Roles      []string   `json:"roles"`
	InvitedBy  InvitedBy  `json:"invited_by"`
	SentAt     time.Time  `json:"sent_at" format:"date-time"`
	ExpiresAt  time.Time  `json:"expires_at" format:"date-time"`
	Status     string     `json:"status" example:"pending"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty" format:"date-time"`
}

// ListInvitationsResponse wraps the admin invitations list.
type ListInvitationsResponse struct {
	Invitations []*Invitation `json:"invitations"`
}

// InvitationPreviewResponse is the privacy-minimal public preview body. It
// carries only a status and an optional UX hint for the primary OAuth button.
// In the MVP suggested_provider is always null and the UI falls back to Google.
type InvitationPreviewResponse struct {
	Status            string  `json:"status" example:"valid"`
	SuggestedProvider *string `json:"suggested_provider"`
}

// OAuthPayload is the provider + signed ID token completed by the frontend.
type OAuthPayload struct {
	Provider string `json:"provider" example:"google"`
	IDToken  string `json:"id_token"`
}

// AcceptInvitationRequest is the public accept body.
type AcceptInvitationRequest struct {
	InvitationToken string       `json:"invitation_token"`
	OAuthPayload    OAuthPayload `json:"oauth_payload"`
}
