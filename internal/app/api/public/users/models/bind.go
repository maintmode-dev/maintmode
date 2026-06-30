package apimodels

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// FromAPIRoleFilter parses the optional `role` query param into a domain role.
// An empty string means "no role filter" and returns the zero role with no
// error. Any non-empty value that is not a known role is rejected.
func FromAPIRoleFilter(role string) (entity.Role, error) {
	switch r := entity.Role(role); r {
	case "":
		return "", nil
	case entity.RoleGuest, entity.RoleEditor, entity.RoleReviewer, entity.RoleAdmin:
		return r, nil
	default:
		return "", fmt.Errorf("unknown role %q", role)
	}
}

// FromAPIRolesFilter parses the optional repeated `roles` query param into
// domain roles (OR filter). An empty input means "no role filter" and returns
// nil with no error. Any value that is not a known role is rejected. Reuses
// FromAPIRoleFilter for per-value validation.
func FromAPIRolesFilter(roles []string) ([]entity.Role, error) {
	if len(roles) == 0 {
		return nil, nil
	}

	out := make([]entity.Role, 0, len(roles))
	for _, raw := range roles {
		if raw == "" {
			continue
		}
		r, err := FromAPIRoleFilter(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// ToAPIUser maps a domain user to its API representation. activeAdminCount is the
// system-wide number of non-blocked admins; is_last_admin is true when this user
// is an active admin and the only one left. connectedProviders are the OAuth
// providers linked to the user (provider ASC); oauth_provider is the primary
// provider (entity.PrimaryOAuthProvider) — "unknown" when none — as in MeResponse.
func ToAPIUser(u *entity.User, activeAdminCount int64, connectedProviders []entity.OAuthProvider) *User {
	connected := lo.Map(connectedProviders, func(p entity.OAuthProvider, _ int) string {
		return string(p)
	})

	return &User{
		ID:                 u.ID,
		Email:              u.Email,
		DisplayName:        u.Name,
		OAuthProvider:      string(entity.PrimaryOAuthProvider(connectedProviders)),
		ConnectedProviders: connected,
		Roles: lo.Map(u.Roles, func(item entity.Role, _ int) string {
			return string(item)
		}),
		CreatedAt: u.CreatedAt,
		// last_seen_at is not tracked yet (out of scope for RUK-150).
		LastSeenAt:  nil,
		IsLastAdmin: u.IsActiveAdmin() && activeAdminCount == 1,
		BlockedAt:   u.BlockedAt,
	}
}

func ToAPIUsers(users []*entity.User, activeAdminCount int64, providersByUser map[uuid.UUID][]entity.OAuthProvider) []*User {
	return lo.Map(users, func(u *entity.User, _ int) *User {
		return ToAPIUser(u, activeAdminCount, providersByUser[u.ID])
	})
}
