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
// A link runs in three steps. /start checks the ticket (verifyLinkTicket); the
// callback spends it and parks what the provider vouched for behind a one-time
// link code (parkLink); and only the account owner's own session, redeeming that
// code with the dance's browser binding, attaches the identity (CompleteLink).
//
// The third step is the point. The callback is a browser navigation that proves
// nothing about who is driving the browser: completing there let anyone who
// minted a ticket send the /start URL to a colleague and have the colleague's
// provider account attached to the sender's profile.

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

// parkLink spends the ticket and parks the identity the provider vouched for
// behind a one-time link code, for the owner's session to redeem.
//
// No token pair and no sign-in code: the person already has a session, which is
// how they minted the ticket in the first place, and which is what has to
// complete the link.
func (s *Service) parkLink(
	ctx context.Context,
	provider entity.AuthMethod,
	linkTicket string,
	claims *entity.OAuthIDTokenClaims,
	binding string,
	meta *entity.AuditMetadata,
) (*entity.DanceOutcome, error) {
	// GETDEL: /start only peeked, so this is what makes a captured cookie
	// useless the second time.
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
	// it -- but the check is repeated because this is what decides which
	// provider the identity is parked under, and a guard that only exists
	// upstream is one refactor away from not existing.
	if intent.Provider != provider {
		xlog.Warn(ctx, "link ticket names another provider",
			xfield.String("minted_for", string(intent.Provider)))
		s.publishLinkRefused(ctx, meta)

		return nil, apperr.ErrLinkTicketUnusable
	}

	code, err := newDanceSecret()
	if err != nil {
		return nil, fmt.Errorf("mint one-time link code: %w", err)
	}

	if err := s.danceCodes.PutLinkCode(ctx, code, entity.PendingLink{
		UserID:   intent.UserID,
		Provider: provider,
		Claims:   *claims,
		Binding:  binding,
	}); err != nil {
		return nil, fmt.Errorf("store one-time link code: %w", err)
	}

	return &entity.DanceOutcome{LinkCode: code}, nil
}

// refuseLink turns a LinkIdentity failure into the error CompleteLink answers
// with, and records it.
//
// A gone or blocked user is ErrLinkCodeInvalid rather than left as-is:
// ErrUserBlocked would map to a 401, which tells the BFF the caller's own
// session is dead. On a link the condition is that this code can no longer be
// used.
//
// The conflict sentinels pass through untouched: they map to 409, and
// rewrapping them would lose the distinction the audit row keeps.
func (s *Service) refuseLink(
	ctx context.Context,
	userID uuid.UUID,
	meta *entity.AuditMetadata,
	err error,
) error {
	if errors.Is(err, apperr.ErrUserNotFound) || errors.Is(err, apperr.ErrUserBlocked) {
		xlog.Warn(ctx, "link refused: the account is gone or blocked", xfield.Error(err))
		s.publishLinkRefused(ctx, meta)

		// %v, not %w: the cause must not reach the mapper, where ErrUserBlocked
		// would answer 401 and sign the caller out.
		return fmt.Errorf("%w: %v", apperr.ErrLinkCodeInvalid, err) //nolint:errorlint // see above
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
