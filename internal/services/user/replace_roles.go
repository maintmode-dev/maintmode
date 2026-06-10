package user

import (
	"context"
	"errors"
	"slices"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ReplaceRoles replaces all roles for a user. Validates every role.
func (s *Service) ReplaceRoles(ctx context.Context, cmd *entity.ReplaceRolesCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.ReplaceRoles")
	defer span.End()

	// Lockout protection (validated server-side, not just in the UI): an actor
	// cannot replace their own role set — a replace can silently drop their admin
	// role and lock them out. Mirrors the self-block / self-revoke guards.
	if cmd.Actor != nil && cmd.Actor.ID == cmd.UserID {
		return apperr.ErrSelfRevoke
	}

	var oldRoles []entity.Role
	user, err := s.updateWithApply(ctx, cmd.UserID, func(ctx context.Context, user *entity.User) error {
		newRoles := lo.FilterMap(cmd.Roles, func(item entity.Role, _ int) (entity.Role, bool) {
			return item, item.Valid(ctx)
		})
		// deduplicated roles
		newRoles = lo.Uniq(newRoles)

		slices.Sort(user.Roles)
		slices.Sort(newRoles)
		oldRoles = slices.Clone(user.Roles)

		if slices.Equal(user.Roles, newRoles) {
			xlog.Warn(ctx, "roles not changed",
				xfield.String("user_id", cmd.UserID.String()),
				xfield.Any("user roles", user.Roles),
				xfield.Any("roles", cmd.Roles),
			)
			return apperr.ErrNotChanged
		}

		user.Roles = newRoles
		return nil
	})
	if err != nil {
		if errors.Is(err, apperr.ErrNotChanged) {
			return nil
		}
		xlog.Error(ctx, "failed to replace roles", xfield.Error(err))
		return err
	}

	s.auditorSrv.LogChangeRoles(ctx, entity.AuditEventRoleReplaced, cmd.Actor, user, entity.AuditRolesChange{
		Roles:   user.Roles,
		Added:   lo.Without(user.Roles, oldRoles...),
		Removed: lo.Without(oldRoles, user.Roles...),
	})

	return nil
}
