package invitation

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xcripto"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// invitationTokenBytes is the entropy of the raw invitation token (256 bits),
// matching the refresh-token generator.
const invitationTokenBytes = 32

// Create issues a new invitation for an email with pre-assigned roles and
// "sends" the email (stub: logs the link).
//
//   - Rejects emails that already belong to a user (409): existing users get
//     roles via /roles/assign, not invitations.
//   - One active pending invitation per email: a still-pending row for the same
//     email is revoked first (so the partial-unique index never blocks the new
//     invite), then the new invitation is inserted — all in one transaction.
func (s *Service) Create(ctx context.Context, cmd *entity.CreateInvitationCmd) (*entity.Invitation, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.Create",
		xfield.String("email", cmd.Email),
	)
	defer span.End()

	if len(cmd.Roles) == 0 {
		return nil, fmt.Errorf("%w: roles must not be empty", apperr.ErrValidation)
	}
	for _, role := range cmd.Roles {
		if !role.Valid(ctx) {
			return nil, apperr.ErrInvalidRole
		}
	}

	// Reject invitations to an already-registered email. This is a best-effort
	// pre-check, not a hard guarantee: there is no cross-table constraint tying
	// user_invitations to users, so a user created between this check and the
	// insert is not caught here. The accept flow is the real safety net — it
	// resolves to the existing user by OAuth identity rather than duplicating one.
	exist, err := s.userSrv.GetByEmail(ctx, cmd.Email)
	if err != nil && !errors.Is(err, apperr.ErrUserNotFound) {
		xlog.Error(ctx, "check existing user failed", xfield.Error(err))
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if err == nil && exist != nil {
		return nil, apperr.ErrUserAlreadyExists
	}

	raw, hash, err := xcripto.GenerateToken(invitationTokenBytes)
	if err != nil {
		xlog.Error(ctx, "generate invitation token failed", xfield.Error(err))
		return nil, err
	}

	var created *entity.Invitation
	err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		now := xtime.UTCNow()
		// Revoke any still-pending invitation for this email so the new one can
		// take the single active-pending slot. The row is locked FOR UPDATE.
		if err := s.revokeActiveInvitation(ctx, now, cmd.Email); err != nil {
			xlog.Error(ctx, "revoke active pending invitation failed", xfield.Error(err))
			return err
		}

		// Seats-cap guard: a seat-role invite reserves a seat from send time
		// (Variant A), so enforce the cap before inserting. The row does not exist
		// yet, so occupied+1 is unconditionally correct. A pure-guest invite takes
		// no seat and skips the guard. Runs after the stale revoke (which frees the
		// prior pending seat for this email) so re-inviting the same address never
		// double-counts. It also runs after that revoke's FOR UPDATE, keeping the
		// row-lock → advisory-lock order.
		if entity.RoleOccupiesSeat(entity.HighestRole(cmd.Roles)) {
			if err := s.seatGuard.EnsureSeatAvailable(ctx); err != nil {
				return err
			}
		}

		created, err = s.store.Create(ctx, &entity.Invitation{
			Email:       cmd.Email,
			Roles:       cmd.Roles,
			TokenHash:   hash,
			Status:      entity.InvitationStatusPending,
			InvitedByID: cmd.Actor.ID,
			ExpiresAt:   now.Add(s.ttl),
			SentAt:      now,
		})
		if err != nil {
			return fmt.Errorf("create invitation: %w", err)
		}

		// Enqueue the email inside the tx (transactional outbox): the goque_task
		// insert commits atomically with the invitation, and the auth processor
		// performs the real SMTP send off the request path. A failed enqueue
		// rolls back the invitation so we never persist an unsendable invite.
		return s.sendInvitationEmail(ctx, created, s.buildAcceptLink(raw))
	})
	if err != nil {
		xlog.Error(ctx, "create invitation failed", xfield.Error(err))
		return nil, err
	}

	return created, nil
}

// buildAcceptLink renders the frontend accept-invite URL carrying the raw token.
func (s *Service) buildAcceptLink(rawToken string) string {
	return fmt.Sprintf("%s/accept-invite?token=%s", s.frontendURL, url.QueryEscape(rawToken))
}

func (s *Service) revokeActiveInvitation(ctx context.Context, now time.Time, email string) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.revokeActiveInvitation")
	defer span.End()

	// GetActivePendingByEmail locks row via FOR UPDATE.
	stale, err := s.store.GetActivePendingByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperr.ErrInvitationNotFound) {
			// no active pending invitation — nothing to revoke
			return nil
		}
		return fmt.Errorf("lookup active pending invitation: %w", err)
	}

	if err := s.store.SetRevoked(ctx, stale.ID, now); err != nil {
		return fmt.Errorf("revoke stale pending invitation: %w", err)
	}

	return nil
}
