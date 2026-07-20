package user

import (
	"context"
	"errors"
	"slices"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/audit"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// AssignRoles adds one or more roles to the user, unioned with the roles they
// already hold, in a single transaction. It validates every role and returns
// the user with its resulting role set. Assigning roles the user already has is
// a no-op (the user is returned unchanged).
func (s *Service) AssignRoles(ctx context.Context, cmd *entity.AssignRolesCmd) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.AssignRoles")
	defer span.End()

	for _, role := range cmd.Roles {
		if !role.Valid(ctx) {
			return nil, apperr.ErrInvalidRole
		}
	}

	var added []entity.Role
	user, err := s.updateWithApply(ctx, cmd.UserID, func(ctx context.Context, user *entity.User) error {
		added = lo.Filter(cmd.Roles, func(role entity.Role, _ int) bool {
			return !slices.Contains(user.Roles, role)
		})
		if len(added) == 0 {
			xlog.Warn(ctx, "roles already assigned",
				xfield.String("user_id", cmd.UserID.String()),
				xfield.Any("user roles", user.Roles),
				xfield.Any("roles", cmd.Roles),
			)
			return apperr.ErrNotChanged
		}

		user.Roles = append(slices.Clone(user.Roles), added...)
		return nil
	})
	if err != nil {
		// Nothing to add — the user already holds every requested role. Return
		// the current user so callers can rely on a non-nil result.
		if errors.Is(err, apperr.ErrNotChanged) {
			return s.GetByID(ctx, cmd.UserID)
		}
		xlog.Error(ctx, "failed to assign roles", xfield.Error(err))
		return nil, err
	}

	s.publishAudit(ctx, audit.RolesChanged{
		Actor:  cmd.Actor,
		Target: user,
		Kind:   audit.RolesAssigned,
		Change: audit.RolesChange{Roles: added},
	})

	return user, nil
}
