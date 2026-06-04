package invitation

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Revoke invalidates a pending invitation's link. Revoking an already-revoked
// invitation is idempotent (no-op success). An accepted invitation cannot be
// revoked — the user already exists.
func (s *Service) Revoke(ctx context.Context, cmd *entity.RevokeInvitationCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.Revoke",
		xfield.String("invitation_id", cmd.ID.String()),
	)
	defer span.End()

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		inv, err := s.store.GetForUpdateByID(ctx, cmd.ID)
		if err != nil {
			return err
		}

		switch inv.Status {
		case entity.InvitationStatusRevoked:
			return nil // idempotent
		case entity.InvitationStatusAccepted:
			return apperr.ErrInvitationNotPending
		}

		if err := s.store.SetRevoked(ctx, inv.ID, xtime.UTCNow()); err != nil {
			return fmt.Errorf("set revoked: %w", err)
		}
		return nil
	})
	if err != nil {
		xlog.Error(ctx, "revoke invitation failed", xfield.Error(err))
		return err
	}

	return nil
}
