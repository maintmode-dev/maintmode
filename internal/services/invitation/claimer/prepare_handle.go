package claimer

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/metrics"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// PrepareHandle parks the invitation a raw token names behind an opaque handle,
// so the dance can carry it across the provider round trip without the token
// leaving this backend.
//
// It reports success for a token that names NOTHING, and that is the point. An
// error here would reach the browser as a failed /start, which distinguishes a
// live invitation token from a dead one before the caller has authenticated
// with anything -- an oracle for grinding tokens against a public endpoint.
// Instead the handle is minted either way and simply redeems to nothing in
// phase 0, after the provider round trip, where the refusal is indistinguishable
// from an ordinary uninvited one.
//
// A store or database failure IS returned: those are not "this invitation does
// not exist", and swallowing them would let a broken store silently degrade
// every invited sign-in into an ordinary refused one.
func (c *Claimer) PrepareHandle(ctx context.Context, handle, invitationToken string) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.InvitationClaimer.PrepareHandle")
	defer span.End()

	// Which invitation the handle will name. uuid.Nil for a token that names
	// none — deliberately, and see the doc comment: the handle is stored either
	// way, so /start does identical work for a live token and a dead one. A
	// dead one simply redeems to nothing in phase 0, after the provider round
	// trip, where the refusal teaches nothing an uninvited one would not.
	invitationID := uuid.Nil

	inv, err := c.store.GetByTokenHash(ctx, xhash.HashSha256([]byte(invitationToken)))
	switch {
	case err == nil:
		invitationID = inv.ID
	case errors.Is(err, apperr.ErrInvitationNotFound):
		xlog.Warn(ctx, "prepare handle: token names no invitation")
	default:
		xlog.Error(ctx, "prepare handle: lookup failed", xfield.Error(err))

		return fmt.Errorf("get invitation by token: %w", err)
	}

	// One store call on both paths, which is what makes "the same work either
	// way" true by construction rather than by keeping two copies in step.
	if err := c.danceHandles.PutInvitationHandle(ctx, handle, invitationID); err != nil {
		xlog.Error(ctx, "prepare handle: store failed", xfield.Error(err))
		metrics.InvitationHandleStoreFailure(ctx)

		return fmt.Errorf("put invitation handle: %w", err)
	}

	return nil
}
