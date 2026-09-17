package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// The LINK half of the OAuth dance: attaching an additional sign-in method to
// an account that already exists.
//
// It is a separate file rather than a separate concern -- these methods are on
// the same Service and run inside the same dance -- because the two halves
// answer different questions. oauth_dance_state.go establishes WHO the browser
// is; this establishes what an already-known account may gain. The sign-in half
// mints a session and knows nothing about tickets; this one mints nothing and
// spends a ticket.
//
// The split is by reading order, not by boundary: nothing here is reachable
// except through StartDance and CompleteDance next door, and moving it changed
// no logic.

// verifyLinkTicket checks that a presented ticket exists and was minted for this
// provider, without spending it.
//
// The provider check is what the intent's Provider field is for: without it a
// ticket minted for one instance could be diverted into linking an identity from
// another, which is not what its owner asked for.
//
// A store FAILURE is a refusal, not a miss. Reading one as "no ticket" would
// downgrade the request to an ordinary sign-in, and the person who asked to link
// their account would instead be signed in as whichever provider account the
// browser happened to be holding.
func (s *Service) verifyLinkTicket(
	ctx context.Context,
	ticket string,
	provider entity.AuthMethod,
) error {
	if ticket == "" {
		return nil
	}

	intent, err := s.danceCodes.PeekLinkTicket(ctx, ticket)
	if err != nil {
		xlog.Error(ctx, "link ticket lookup failed", xfield.Error(err))

		return fmt.Errorf("peek link ticket: %w", err)
	}

	// Unknown, expired or already spent -- one answer, as everywhere else in this
	// flow: telling them apart would confirm half a guess.
	if intent == nil {
		xlog.Warn(ctx, "link ticket redeemed nothing")

		return apperr.ErrLinkTicketUnusable
	}

	if intent.Provider != provider {
		xlog.Warn(ctx, "link ticket was minted for another provider",
			xfield.String("minted_for", string(intent.Provider)))

		return apperr.ErrLinkTicketUnusable
	}

	return nil
}

// completeLink attaches the authenticated identity to the account the ticket
// names.
//
// No token pair and no one-time code: the person already has a session, which is
// how they minted the ticket in the first place.
func (s *Service) completeLink(
	ctx context.Context,
	provider entity.AuthMethod,
	linkTicket string,
	claims *entity.OAuthIDTokenClaims,
	meta *entity.AuditMetadata,
) (*entity.DanceOutcome, error) {
	// GETDEL: the single spend in the whole flow. /start only peeked, so this is
	// what makes a captured cookie useless the second time.
	intent, err := s.danceCodes.ConsumeLinkTicket(ctx, linkTicket)
	if err != nil {
		// A store failure is NOT a miss. Falling through to sign-in here would
		// mint a session for whichever provider account was just authenticated --
		// the exact hazard /start refuses one step earlier.
		xlog.Error(ctx, "link ticket redeem failed", xfield.Error(err))
		// Audited like every other refusal on this path, so "every link exit
		// leaves a row" holds by construction rather than in four branches out of
		// five. It names no account -- nothing redeemed -- but a 302 leaves no
		// other trace, and this is the refusal that signals infrastructure rather
		// than user behavior.
		s.publishLinkRefused(ctx, meta)

		return nil, fmt.Errorf("consume link ticket: %w", err)
	}

	if intent == nil {
		xlog.Warn(ctx, "link callback redeemed nothing")
		s.publishLinkRefused(ctx, meta)

		return nil, apperr.ErrLinkTicketUnusable
	}

	// A ticket minted for another provider cannot arrive here -- /start refuses
	// it -- but the check is repeated because this is where the identity is
	// actually attached, and a guard that only exists upstream is one refactor
	// away from not existing.
	if intent.Provider != provider {
		xlog.Warn(ctx, "link ticket names another provider",
			xfield.String("minted_for", string(intent.Provider)))
		s.publishLinkRefused(ctx, meta)

		return nil, apperr.ErrLinkTicketUnusable
	}

	if err := s.usersSrv.LinkIdentity(ctx, intent.UserID, provider, claims); err != nil {
		return nil, s.refuseLink(ctx, intent.UserID, meta, err)
	}

	s.publishLinked(ctx, intent.UserID, meta)

	return &entity.DanceOutcome{Linked: true}, nil
}

// refuseLink turns a LinkIdentity failure into the error the redirect maps, and
// records it.
//
// A gone or blocked user is wrapped in ErrLinkTicketUnusable rather than left
// as-is: danceFailureCode is a free function over one error and cannot know
// which branch produced it, so ErrUserBlocked would reach the arm that answers
// access_denied -- "you may not sign in", which is not what happened. On a link
// the condition is that this ticket can no longer be used.
//
// The conflict sentinels pass through untouched: they already map to
// link_conflict, and rewrapping them would lose the distinction the audit row
// keeps.
func (s *Service) refuseLink(
	ctx context.Context,
	userID uuid.UUID,
	meta *entity.AuditMetadata,
	err error,
) error {
	if errors.Is(err, apperr.ErrUserNotFound) || errors.Is(err, apperr.ErrUserBlocked) {
		xlog.Warn(ctx, "link refused: the account is gone or blocked", xfield.Error(err))
		s.publishLinkRefused(ctx, meta)

		return fmt.Errorf("%w: %w", apperr.ErrLinkTicketUnusable, err)
	}

	xlog.Warn(ctx, "link refused", xfield.Error(err))
	s.publishLinkRefusedFor(ctx, userID, meta, entity.AuditFailureLinkConflict)

	return fmt.Errorf("link identity: %w", err)
}

// publishLinked records a completed link.
//
// Through the generic publishAudit rather than publishLoginFailure, which is
// hard-wired to audit.LoginFailed: a link filed under login.* would corrupt the
// facet where a run of failures reads as credential guessing.
func (s *Service) publishLinked(ctx context.Context, userID uuid.UUID, meta *entity.AuditMetadata) {
	s.publishAudit(ctx, audit.ProviderLinked{
		User: &entity.User{ID: userID},
		Meta: meta,
	})
}

// publishLinkRefused records a refusal whose account could not be resolved --
// a ticket that redeemed to nothing, or a store that could not answer, names
// nobody.
//
// The reason is fixed rather than a parameter: every refusal that reaches here
// is the same one, "this ticket cannot be used". A conflict names an account and
// goes through publishLinkRefusedFor instead.
func (s *Service) publishLinkRefused(ctx context.Context, meta *entity.AuditMetadata) {
	s.publishLinkRefusedFor(ctx, uuid.Nil, meta, entity.AuditFailureLinkUnusable)
}

// publishLinkRefusedFor records a refusal against a known account.
//
// The metadata is COPIED before the reason is stamped, for the same reason
// publishLoginFailure copies it: callers reuse one value across branches, and
// mutating it would leak one branch's reason into another's row.
func (s *Service) publishLinkRefusedFor(
	ctx context.Context,
	userID uuid.UUID,
	meta *entity.AuditMetadata,
	reason entity.AuditFailureReason,
) {
	s.publishAudit(ctx, audit.ProviderLinked{
		User: &entity.User{ID: userID},
		Meta: metaWithReason(meta, reason),
	})
}
