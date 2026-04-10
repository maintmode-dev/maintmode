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

	user, err := s.updateWithApply(ctx, cmd.UserID, func(ctx context.Context, user *entity.User) error {
		newRoles := lo.FindUniquesBy(cmd.Roles, func(item entity.Role) bool {
			return item.Valid(ctx)
		})

		slices.Sort(user.Roles)
		slices.Sort(newRoles)

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

	s.auditorSrv.LogChangeRoles(ctx, entity.AuditEventRoleReplaced, cmd.Actor, user, cmd.Roles)

	return nil
}
