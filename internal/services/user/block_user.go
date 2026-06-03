package user

import (
	"context"
	"errors"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// BlockUser blocks a user: sets blocked_at, revokes their refresh tokens and
// audits the action. Idempotent — blocking an already-blocked user is a no-op
// that returns nil (the API maps this to 204).
//
// Lockout protection (validated server-side, not just in the UI):
//   - an admin cannot block themselves (ErrSelfBlock);
//   - the last active admin cannot be blocked (ErrLastAdmin).
func (s *Service) BlockUser(ctx context.Context, cmd *entity.BlockUserCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.BlockUser")
	defer span.End()

	if cmd.Actor != nil && cmd.Actor.ID == cmd.UserID {
		return apperr.ErrSelfBlock
	}

	user, err := s.updateWithApply(ctx, cmd.UserID, func(ctx context.Context, user *entity.User) error {
		if user.IsBlocked() {
			return apperr.ErrNotChanged
		}

		if err := s.ensureNotLastActiveAdmin(ctx, user); err != nil {
			return err
		}

		now := xtime.UTCNow()
		user.BlockedAt = &now
		return nil
	})
	if err != nil {
		if errors.Is(err, apperr.ErrNotChanged) {
			return nil
		}
		xlog.Error(ctx, "failed to block user", xfield.Error(err))
		return err
	}

	// Revoke the target's sessions after the block commits. Best-effort: a failure
	// here is logged but does not fail the block — the user is already blocked and
	// can no longer authenticate new access tokens via the RBAC layer.
	if err := s.tokenRevoker.RevokeRefreshTokenByUserID(ctx, cmd.UserID); err != nil {
		xlog.Error(ctx, "failed to revoke refresh tokens on block", xfield.Error(err))
	}

	s.auditorSrv.LogBlockUser(ctx, entity.AuditEventUserBlocked, cmd.Actor, user)

	return nil
}
