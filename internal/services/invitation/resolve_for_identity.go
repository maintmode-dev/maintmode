package invitation

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/metrics"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// ResolveForIdentity is phase 0 of an invited dance: it redeems the handle the
// browser carried, checks the invitation is live, and enforces the email match
// against the provider's verified claims.
//
// It runs BEFORE the user is created, which is the whole reason it is separate
// from ClaimForUser. A refusal here means no account exists and no session was
// issued — the alternative, checking after sign-in, would mean the anti-takeover
// guard fires against a user who is already logged in, and on an instance with
// no admins yet that user would already hold admin.
//
// It grants nothing and writes nothing beyond consuming the handle.
//
// Every refusal answers with apperr.ErrInvalidInvitation or
// apperr.ErrEmailMismatch, wrapped with %w so errors.Is survives to the
// redirect mapper. The two are distinguishable on purpose and no further:
// "wrong account" is the one failure a legitimate person can fix themselves,
// while everything about the invitation's existence and state collapses into
// one answer.
//
// There is deliberately NO seat check here. The seat was reserved when the
// invitation was created, and ListPendingRoles counts live pending invitations
// as occupied — so re-checking at accept time would count this invitee twice
// and, at exactly the cap, refuse them the seat that is already theirs.
func (s *Service) ResolveForIdentity(
	ctx context.Context,
	handle string,
	claims *entity.OAuthIDTokenClaims,
) (*entity.ResolvedInvitation, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.ResolveForIdentity")
	defer span.End()

	// No handle store means no dance is configured on this instance, so there is
	// nothing an invited dance could resolve. Refusing is the fail-closed
	// answer; the alternative is a nil dereference on a public route.
	if s.danceHandles == nil {
		return nil, apperr.ErrInvalidInvitation
	}

	invitationID, err := s.danceHandles.ConsumeInvitationHandle(ctx, handle)
	if err != nil {
		// A store that cannot answer must not fall through to "no invitation":
		// the caller reads that as an uninvited dance and refuses, which is the
		// fail-closed direction, but the operator needs the real cause.
		xlog.Error(ctx, "resolve invitation: consume handle failed", xfield.Error(err))
		metrics.InvitationHandleStoreFailure(ctx)

		return nil, fmt.Errorf("consume invitation handle: %w", err)
	}

	// Nothing behind the handle: an unknown, forged or already-redeemed one.
	// /start stores a handle whenever the parameter is present without resolving
	// the token, so this is the ordinary path for a dead invitation.
	if invitationID == nil {
		xlog.Warn(ctx, "resolve invitation: handle redeemed nothing")

		return nil, apperr.ErrInvalidInvitation
	}

	inv, err := s.store.GetByID(ctx, *invitationID)
	if err != nil {
		if errors.Is(err, apperr.ErrInvitationNotFound) {
			return nil, apperr.ErrInvalidInvitation
		}
		xlog.Error(ctx, "resolve invitation: lookup failed", xfield.Error(err))

		return nil, fmt.Errorf("get invitation: %w", err)
	}

	// Any non-live status collapses to a single "invalid" — never leak which.
	if inv.EffectiveStatus(xtime.UTCNow()) != entity.InvitationStatusPending {
		xlog.Warn(ctx, "resolve invitation: invitation is not pending")

		return nil, apperr.ErrInvalidInvitation
	}

	// Anti-takeover guard: the account signing in must be the invited address.
	// Shared with the id_token accept path rather than reimplemented — a second
	// copy is how this check once got disabled in one place and not the other.
	if !emailMatchesIgnoreCase(ctx, claims.Email, inv.Email) {
		xlog.Warn(ctx, "resolve invitation: provider email does not match invitation")

		return nil, apperr.ErrEmailMismatch
	}

	return &entity.ResolvedInvitation{ID: inv.ID, Roles: inv.Roles}, nil
}
