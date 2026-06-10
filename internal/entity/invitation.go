package entity

import (
	"time"

	"github.com/google/uuid"
)

// InvitationStatus is the lifecycle state of a user invitation.
//
// pending, accepted and revoked are stored in the DB. expired is DERIVED at
// read time from (status=pending AND expires_at < now) and is never persisted —
// see Invitation.EffectiveStatus.
type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusRevoked  InvitationStatus = "revoked"
	InvitationStatusExpired  InvitationStatus = "expired" // derived only, never stored
)

// InvitationPreviewStatus is the minimal, privacy-safe status reported to the
// public preview endpoint. It adds "valid" (a live pending invitation) and
// "invalid" (token not found / unverifiable) on top of the lifecycle states,
// and intentionally carries no other invitation data.
type InvitationPreviewStatus string

const (
	InvitationPreviewStatusValid    InvitationPreviewStatus = "valid"
	InvitationPreviewStatusInvalid  InvitationPreviewStatus = "invalid"
	InvitationPreviewStatusExpired  InvitationPreviewStatus = "expired"
	InvitationPreviewStatusAccepted InvitationPreviewStatus = "accepted"
	InvitationPreviewStatusRevoked  InvitationPreviewStatus = "revoked"
)

// Invitation is an admin-issued invitation for a new user to onboard via OAuth.
// TokenHash is the sha256 of the raw token; the raw token only ever lives in
// the email link and is never returned by any endpoint.
type Invitation struct {
	ID          uuid.UUID
	Email       string
	Roles       []Role
	TokenHash   string
	Status      InvitationStatus // stored status: pending|accepted|revoked
	InvitedByID uuid.UUID
	ExpiresAt   time.Time
	SentAt      time.Time
	AcceptedAt  *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}

// EffectiveStatus resolves the externally-visible status, deriving "expired"
// from a pending invitation whose expiry has passed.
func (i *Invitation) EffectiveStatus(now time.Time) InvitationStatus {
	if i.Status == InvitationStatusPending && i.ExpiresAt.In(time.UTC).Before(now.In(time.UTC)) {
		return InvitationStatusExpired
	}
	return i.Status
}

// PreviewStatus maps the effective status to the privacy-safe preview status.
// A live pending invitation reports "valid".
func (i *Invitation) PreviewStatus(now time.Time) InvitationPreviewStatus {
	switch i.EffectiveStatus(now) {
	case InvitationStatusPending:
		return InvitationPreviewStatusValid
	case InvitationStatusExpired:
		return InvitationPreviewStatusExpired
	case InvitationStatusAccepted:
		return InvitationPreviewStatusAccepted
	case InvitationStatusRevoked:
		return InvitationPreviewStatusRevoked
	default:
		return InvitationPreviewStatusInvalid
	}
}

// InvitationListItem is an admin-view invitation joined with its inviter's
// resolved profile, for GET /users/invitations.
type InvitationListItem struct {
	Invitation *Invitation
	Inviter    *UserSummary
}

// ParseInvitationStatusFilter validates the ?status= list filter. The empty
// string means "all statuses" and returns ("", true).
func ParseInvitationStatusFilter(s string) (InvitationStatus, bool) {
	switch InvitationStatus(s) {
	case "",
		InvitationStatusPending,
		InvitationStatusAccepted,
		InvitationStatusRevoked,
		InvitationStatusExpired:
		return InvitationStatus(s), true
	default:
		return "", false
	}
}

// CreateInvitationCmd is the admin request to invite a new user by email.
type CreateInvitationCmd struct {
	Actor *User
	Email string
	Roles []Role
}

// ListInvitationsCmd filters the admin invitations list. Status "" means all.
// Now is the reference time used to derive the pending/expired boundary; the
// service fills it from its clock so the storage filter stays consistent and
// testable.
type ListInvitationsCmd struct {
	Status InvitationStatus
}

// RevokeInvitationCmd revokes a pending invitation, invalidating its link.
type RevokeInvitationCmd struct {
	Actor *User
	ID    uuid.UUID
}

// ResendInvitationCmd re-sends a pending invitation, rotating the token and
// extending the expiry.
type ResendInvitationCmd struct {
	Actor *User
	ID    uuid.UUID
}

// AcceptInvitationCmd is the public request to accept an invitation: the raw
// token from the email link plus the OAuth payload completed by the frontend.
type AcceptInvitationCmd struct {
	Token    string
	Provider OAuthProvider
	IDToken  string
	ClientIP string
}
