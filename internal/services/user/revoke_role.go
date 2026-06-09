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

// RevokeRole removes a role from the user.
func (s *Service) RevokeRole(ctx context.Context, cmd *entity.RevokeRoleCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.RevokeRole")
	defer span.End()

	if !cmd.Role.Valid(ctx) {
		return apperr.ErrInvalidRole
	}

	// Lockout protection (validated server-side, not just in the UI): an actor
	// cannot revoke a role from themselves — that is how admins lock themselves
	// out of the system. Mirrors the self-block guard in BlockUser.
	if cmd.Actor != nil && cmd.Actor.ID == cmd.UserID {
		return apperr.ErrSelfRevoke
	}

	user, err := s.updateWithApply(ctx, cmd.UserID, func(ctx context.Context, user *entity.User) error {
		idx := slices.Index(user.Roles, cmd.Role)
		if idx == -1 {
			xlog.Warn(ctx, "role not present",
				xfield.String("user_id", cmd.UserID.String()),
				xfield.Any("user roles", user.Roles),
				xfield.Any("role", cmd.Role),
			)
			return apperr.ErrNotChanged
		}

		// Lockout protection: refuse to strip the admin role from the last active
		// admin. Validated server-side — the UI guard is not authoritative.
		if cmd.Role == entity.RoleAdmin {
			if err := s.ensureNotLastActiveAdmin(ctx, user); err != nil {
				return err
			}
		}

		user.Roles = slices.Delete(user.Roles, idx, idx+1)
		return nil
	})
	if err != nil {
		if errors.Is(err, apperr.ErrNotChanged) {
			return nil
		}
		xlog.Error(ctx, "failed to revoke role", xfield.Error(err))
		return err
	}

	s.auditorSrv.LogChangeRoles(ctx, entity.AuditEventRoleRevoked, cmd.Actor, user, []entity.Role{cmd.Role})

	return nil
}
