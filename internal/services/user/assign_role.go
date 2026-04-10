package user

import (
	"context"
	"errors"
	"slices"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// AssignRole adds a role to the user
func (s *Service) AssignRole(ctx context.Context, cmd *entity.AssignRoleCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.AssignRole")
	defer span.End()

	if !cmd.Role.Valid(ctx) {
		return apperr.ErrInvalidRole
	}

	user, err := s.updateWithApply(ctx, cmd.UserID, func(ctx context.Context, user *entity.User) error {
		if slices.Contains(user.Roles, cmd.Role) {
			xlog.Warn(ctx, "role already assigned",
				xfield.String("user_id", cmd.UserID.String()),
				xfield.Any("user roles", user.Roles),
				xfield.Any("role", cmd.Role),
			)
			return apperr.ErrNotChanged
		}

		user.Roles = append(user.Roles, cmd.Role)
		return nil
	})
	if err != nil {
		if errors.Is(err, apperr.ErrNotChanged) {
			return nil
		}
		xlog.Error(ctx, "failed to assign role", xfield.Error(err))
		return err
	}

	s.auditorSrv.LogChangeRoles(ctx, entity.AuditEventRoleAssigned, cmd.Actor, user, []entity.Role{cmd.Role})

	return nil
}
