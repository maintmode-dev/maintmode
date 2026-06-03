package apimodels

import (
	"fmt"

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

// ToAPIUser maps a domain user to its API representation. activeAdminCount is the
// system-wide number of non-blocked admins; is_last_admin is true when this user
// is an active admin and the only one left.
func ToAPIUser(u *entity.User, activeAdminCount int64) *User {
	return &User{
		ID:                 u.ID,
		Email:              u.Email,
		DisplayName:        u.Name,
		OAuthProvider:      string(entity.OAuthProviderGoogle),
		ConnectedProviders: []string{},
		Roles: lo.Map(u.Roles, func(item entity.Role, _ int) string {
			return string(item)
		}),
		CreatedAt:   u.CreatedAt,
		LastSeenAt:  nil,
		IsLastAdmin: u.IsActiveAdmin() && activeAdminCount == 1,
		BlockedAt:   u.BlockedAt,
	}
}

func ToAPIUsers(users []*entity.User, activeAdminCount int64) []*User {
	return lo.Map(users, func(u *entity.User, _ int) *User {
		return ToAPIUser(u, activeAdminCount)
	})
}
