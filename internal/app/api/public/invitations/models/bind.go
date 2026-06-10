package apimodels

import (
	"fmt"
	"time"

	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// FromAPIRoles converts API role strings to domain roles, failing on the first
// unknown value.
func FromAPIRoles(roles []string) ([]entity.Role, error) {
	out := make([]entity.Role, 0, len(roles))
	for _, r := range roles {
		role, err := fromAPIRole(r)
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	return out, nil
}

// fromAPIRole validates a role string against the known domain roles. Kept
// local to this API package so the invitations contract does not depend on
// another endpoint's models.
func fromAPIRole(r string) (entity.Role, error) {
	switch role := entity.Role(r); role {
	case entity.RoleGuest, entity.RoleEditor, entity.RoleReviewer, entity.RoleAdmin:
		return role, nil
	default:
		return "", fmt.Errorf("unsupported role: %q", r)
	}
}

func toAPIRoleStrings(roles []entity.Role) []string {
	return lo.Map(roles, func(item entity.Role, _ int) string { return string(item) })
}

// ToAPICreateInvitationResponse maps a created invitation to the 201 body. The
// effective status is derived against now.
func ToAPICreateInvitationResponse(inv *entity.Invitation, now time.Time) *CreateInvitationResponse {
	return &CreateInvitationResponse{
		InvitationID: inv.ID,
		Email:        inv.Email,
		Roles:        toAPIRoleStrings(inv.Roles),
		ExpiresAt:    inv.ExpiresAt,
		Status:       string(inv.EffectiveStatus(now)),
	}
}

// ToAPIInvitation maps an admin-view list item to its API shape.
func ToAPIInvitation(item *entity.InvitationListItem, now time.Time) *Invitation {
	inv := item.Invitation
	return &Invitation{
		ID:         inv.ID,
		Email:      inv.Email,
		Roles:      toAPIRoleStrings(inv.Roles),
		Inviter:    ToAPIUserSummaryFromSummary(item.Inviter),
		SentAt:     inv.SentAt,
		ExpiresAt:  inv.ExpiresAt,
		Status:     string(inv.EffectiveStatus(now)),
		AcceptedAt: inv.AcceptedAt,
	}
}

func ToAPIInvitations(items []*entity.InvitationListItem, now time.Time) []*Invitation {
	return lo.Map(items, func(item *entity.InvitationListItem, _ int) *Invitation {
		return ToAPIInvitation(item, now)
	})
}

// ToAPIUserSummaryFromSummary maps a resolved domain user summary (from the auth
// resolver) to its API shape. Nil-safe: a nil summary maps to nil (null).
func ToAPIUserSummaryFromSummary(u *entity.UserSummary) *UserSummary {
	if u == nil {
		return nil
	}

	return &UserSummary{
		ID:          u.ID,
		DisplayName: u.Name,
		Email:       u.Email,
	}
}
