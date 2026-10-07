package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// CompleteLink attaches the identity a pending link holds to the account of the
// session redeeming it.
//
// Every reason the code cannot be redeemed by THIS caller answers the same
// ErrLinkCodeInvalid: unknown, expired or spent, bound to another browser,
// started by another account, or that account gone or blocked. A conflict on
// the identity itself passes through, because the owner can act on it.
func (s *Service) CompleteLink(ctx context.Context, cmd *entity.CompleteLinkCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.CompleteLink")
	defer span.End()

	// GETDEL: whatever the outcome below, this code is spent, so a wrong guess
	// at the binding or the account costs the guesser the code.
	link, err := s.danceCodes.ConsumeLinkCode(ctx, cmd.LinkCode)
	if err != nil {
		return fmt.Errorf("consume link code: %w", err)
	}

	if link == nil {
		xlog.Warn(ctx, "link code redeemed nothing")
		s.publishLinkRefused(ctx, cmd.Meta)

		return apperr.ErrLinkCodeInvalid
	}

	if !danceBindingProven(link.Binding, cmd.BindingProof) {
		xlog.Warn(ctx, "link code presented without its browser binding")
		s.publishLinkRefusedFor(ctx, link.UserID, cmd.Meta, entity.AuditFailureLinkUnusable)

		return apperr.ErrLinkCodeInvalid
	}

	// The account that minted the ticket must be the one completing the link.
	// This is the check the callback could not make.
	if link.UserID != cmd.UserID {
		xlog.Warn(ctx, "link code redeemed by another account",
			xfield.String("link_owner", link.UserID.String()))
		s.publishLinkRefusedFor(ctx, link.UserID, cmd.Meta, entity.AuditFailureLinkUnusable)

		return apperr.ErrLinkCodeInvalid
	}

	if err := s.usersSrv.LinkIdentity(ctx, link.UserID, link.Provider, &link.Claims); err != nil {
		return s.refuseLink(ctx, link.UserID, cmd.Meta, err)
	}

	s.publishLinked(ctx, link.UserID, cmd.Meta)

	return nil
}
