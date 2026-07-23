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

		// Lockout protection: a replace that strips admin from the last active
		// admin is the same hole as revoking the admin role directly — guard it
		// the same way. The guard only matters when this replace actually removes
		// admin: admin was in the old set and is gone from the new one. A replace
		// that keeps admin, or never had it, cannot shrink the admin count.
		strippingAdmin := slices.Contains(oldRoles, entity.RoleAdmin) &&
			!slices.Contains(newRoles, entity.RoleAdmin)
		if strippingAdmin {
			if err := s.ensureNotLastActiveAdmin(ctx, user); err != nil {
				return err
			}
		}

		// Seats-cap guard: fire only when this replace lifts the user from
		// non-seat to seat. Runs AFTER the last-admin guard so advisory keys are
		// taken in ascending order (admin=1 before seat=2, per advisory_lock.go).
		// The count runs before Update persists newRoles, so occupied+1 excludes
		// this in-flight grant. Replace that keeps/lowers seat status skips it.
		if !entity.RoleOccupiesSeat(entity.HighestRole(oldRoles)) &&
			entity.RoleOccupiesSeat(entity.HighestRole(newRoles)) {
			if err := s.seatGuard.EnsureSeatAvailable(ctx); err != nil {
				return err
			}
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

	s.publishAudit(ctx, audit.RolesChanged{
		Actor:  cmd.Actor,
		Target: user,
		Kind:   audit.RolesReplaced,
		Change: audit.RolesChange{
			Roles:   user.Roles,
			Added:   lo.Without(user.Roles, oldRoles...),
			Removed: lo.Without(oldRoles, user.Roles...),
		},
	})

	return nil
}
