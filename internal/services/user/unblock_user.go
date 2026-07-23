package user

import (
	"context"
	"errors"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// UnblockUser clears blocked_at. Roles are preserved on block, so unblocking
// immediately restores the user's previous access. Idempotent — unblocking an
// active user is a no-op that returns nil (the API maps this to 204).
func (s *Service) UnblockUser(ctx context.Context, cmd *entity.UnblockUserCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.UnblockUser")
	defer span.End()

	user, err := s.updateWithApply(ctx, cmd.UserID, func(ctx context.Context, user *entity.User) error {
		if !user.IsBlocked() {
			return apperr.ErrNotChanged
		}

		// Seats-cap guard: unblocking restores the user's preserved roles, so a
		// seat-role user reclaims a seat. Runs while the user is still blocked
		// (BlockedAt not yet cleared), so ListActiveRoles still excludes them and
		// occupied+1 models the restored seat. Gated behind the IsBlocked no-op
		// check above, so unblocking an already-active user never fires it. A
		// guest holds no seat and skips the guard.
		if entity.RoleOccupiesSeat(entity.HighestRole(user.Roles)) {
			if err := s.seatGuard.EnsureSeatAvailable(ctx); err != nil {
				return err
			}
		}

		user.BlockedAt = nil
		return nil
	})
	if err != nil {
		if errors.Is(err, apperr.ErrNotChanged) {
			return nil
		}
		xlog.Error(ctx, "failed to unblock user", xfield.Error(err))
		return err
	}

	s.publishAudit(ctx, audit.UserUnblocked{
		Actor:  cmd.Actor,
		Target: user,
	})

	return nil
}
