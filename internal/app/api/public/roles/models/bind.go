package apimodels

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

func FromAPIRole(r Role) (entity.Role, error) {
	switch r {
	case RoleGuest:
		return entity.RoleGuest, nil
	case RoleEditor:
		return entity.RoleEditor, nil
	case RoleReviewer:
		return entity.RoleReviewer, nil
	case RoleAdmin:
		return entity.RoleAdmin, nil
	default:
		return "", fmt.Errorf("unsupported role: %s", r)
	}
}

func ToAPIRoles(ctx context.Context, roles []entity.Role) []Role {
	return lo.FilterMap(roles, func(item entity.Role, _ int) (Role, bool) {
		role, err := ToAPIRole(item)
		if err != nil {
			xlog.Error(ctx, "unsupported role", xfield.Error(err))
			return "", false
		}
		return role, true
	})
}

func ToAPIRole(r entity.Role) (Role, error) {
	switch r {
	case entity.RoleGuest:
		return RoleGuest, nil
	case entity.RoleEditor:
		return RoleEditor, nil
	case entity.RoleReviewer:
		return RoleReviewer, nil
	case entity.RoleAdmin:
		return RoleAdmin, nil
	default:
		return "", fmt.Errorf("unsupported role: %s", r)
	}
}
