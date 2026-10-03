package claimer

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// ResolveByToken turns the raw token from an invitation link into the live
// invitation it names, email included.
//
// It is the password path's counterpart to ResolveForIdentity. There is no
// provider to vouch for an address, so the invitation's own email is the
// identity: the link was delivered to that mailbox, and possessing it is the
// proof of ownership the provider's verified claim is everywhere else.
//
// Like ResolveForIdentity it grants nothing and writes nothing, and every
// refusal -- unknown token, expired, revoked, accepted -- collapses into
// apperr.ErrInvalidInvitation.
func (c *Claimer) ResolveByToken(ctx context.Context, rawToken string) (*entity.Invitation, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.InvitationClaimer.ResolveByToken")
	defer span.End()

	if rawToken == "" {
		return nil, apperr.ErrInvalidInvitation
	}

	inv, err := c.store.GetByTokenHash(ctx, xhash.HashSha256([]byte(rawToken)))
	if err != nil {
		if errors.Is(err, apperr.ErrInvitationNotFound) {
			return nil, apperr.ErrInvalidInvitation
		}
		xlog.Error(ctx, "resolve invitation by token: lookup failed", xfield.Error(err))

		return nil, fmt.Errorf("get invitation: %w", err)
	}

	if inv.EffectiveStatus(xtime.UTCNow()) != entity.InvitationStatusPending {
		xlog.Warn(ctx, "resolve invitation by token: invitation is not pending")

		return nil, apperr.ErrInvalidInvitation
	}

	return inv, nil
}
