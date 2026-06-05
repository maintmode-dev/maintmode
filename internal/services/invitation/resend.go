package invitation

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xcripto"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Resend rotates the token and extends the expiry of a live pending invitation,
// then re-sends the email. It works only for pending (not expired) invitations:
//
//   - expired   → 409 (admin must create a fresh invitation)
//   - accepted  → 409
//   - revoked   → 409
func (s *Service) Resend(ctx context.Context, cmd *entity.ResendInvitationCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.Resend",
		xfield.String("invitation_id", cmd.ID.String()),
	)
	defer span.End()

	now := xtime.UTCNow()

	raw, hash, err := xcripto.GenerateToken(invitationTokenBytes)
	if err != nil {
		xlog.Error(ctx, "generate invitation token failed", xfield.Error(err))
		return err
	}

	var resended *entity.Invitation
	err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		inv, err := s.store.GetForUpdateByID(ctx, cmd.ID)
		if err != nil {
			return err
		}

		switch inv.EffectiveStatus(now) {
		case entity.InvitationStatusPending:
			// resendable
		case entity.InvitationStatusExpired:
			return apperr.ErrInvitationExpired
		default:
			return apperr.ErrInvitationNotPending
		}

		resended, err = s.store.Resend(ctx, inv.ID, hash, now.Add(s.ttl), now)
		if err != nil {
			return fmt.Errorf("resend invitation: %w", err)
		}

		// Enqueue inside the tx (transactional outbox), like Create.
		return s.sendInvitationEmail(ctx, resended, s.buildAcceptLink(raw))
	})
	if err != nil {
		xlog.Error(ctx, "resend invitation failed", xfield.Error(err))
		return err
	}

	return nil
}
