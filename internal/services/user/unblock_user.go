package user

import (
	"context"
	"errors"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/eventbus/events"
)

// UnblockUser clears blocked_at. Roles are preserved on block, so unblocking
// immediately restores the user's previous access. Idempotent — unblocking an
// active user is a no-op that returns nil (the API maps this to 204).
func (s *Service) UnblockUser(ctx context.Context, cmd *entity.UnblockUserCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.UnblockUser")
	defer span.End()

	user, err := s.updateWithApply(ctx, cmd.UserID, func(_ context.Context, user *entity.User) error {
		if !user.IsBlocked() {
			return apperr.ErrNotChanged
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

	s.dispatcher.AsyncDispatch(ctx, events.UserUnblocked{
		Actor:  cmd.Actor,
		Target: user,
	})

	return nil
}
