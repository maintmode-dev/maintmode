package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// StartDance mints one dance's secrets, signs its state and builds the
// authorization URL. The handler's job is to put the results in cookies and a
// redirect.
func (s *Service) StartDance(
	ctx context.Context,
	provider entity.AuthMethod,
	invitationToken string,
) (*entity.DanceStart, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.StartDance")
	defer span.End()

	state, err := newDanceSecret()
	if err != nil {
		return nil, fmt.Errorf("mint oauth state: %w", err)
	}

	verifier, err := newDanceSecret()
	if err != nil {
		return nil, fmt.Errorf("mint pkce verifier: %w", err)
	}

	invitationHandle, err := s.mintInvitationHandle(ctx, invitationToken)
	if err != nil {
		return nil, err
	}

	return &entity.DanceStart{
		State:            state,
		StateSignature:   s.danceSigner.Sign(string(provider), state, time.Now().Add(s.danceStateTTL)),
		Verifier:         verifier,
		AuthorizationURL: s.danceGateway.AuthCodeURL(state, verifier),
		TTL:              s.danceStateTTL,
		InvitationHandle: invitationHandle,
	}, nil
}

// mintInvitationHandle turns a raw invitation token into an opaque handle and
// parks the invitation behind it, returning "" when no token was presented.
//
// It does NOT check whether the token resolves to a live invitation. Doing so
// would make /start an oracle: a caller could tell live tokens from dead ones
// before authenticating with anything. A handle is minted and stored for every
// token presented, and a dead one simply redeems to nothing in phase 0 — after
// the provider round trip, where the answer teaches nothing an uninvited
// refusal would not.
//
// The token is handed to the invitation side, which owns the token→invitation
// lookup and the store write. The auth service never learns which invitation a
// handle names, and never holds the raw token beyond this call.
func (s *Service) mintInvitationHandle(ctx context.Context, invitationToken string) (string, error) {
	// No token, or an instance with no invitation side wired: an ordinary dance.
	if invitationToken == "" || s.invitations == nil {
		return "", nil
	}

	handle, err := newDanceSecret()
	if err != nil {
		return "", fmt.Errorf("mint invitation handle: %w", err)
	}

	if err := s.invitations.PrepareHandle(ctx, handle, invitationToken); err != nil {
		// Fail closed. Returning the handle anyway would leave the browser
		// carrying one that redeems to nothing, which reads as "your invitation
		// is invalid" rather than "our store is down" — and refusing here is what
		// keeps a store outage from ever widening account creation.
		return "", fmt.Errorf("store invitation handle: %w", err)
	}

	return handle, nil
}

// RedeemDanceCode trades a one-time opaque code for the pair parked behind it.
// A nil pair with no error means nothing to redeem — unknown, expired or spent,
// deliberately indistinguishable. The consume is atomic, so N concurrent
// redemptions yield one winner.
func (s *Service) RedeemDanceCode(ctx context.Context, code string) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.RedeemDanceCode")
	defer span.End()

	pair, err := s.danceCodes.ConsumeCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("consume one-time dance code: %w", err)
	}

	return pair, nil
}

// CompleteDance turns a callback into a one-time code the frontend can redeem,
// or an error saying why it could not.
//
// The ORDER below is the security property: the provider is resolved before the
// signature is re-derived because it is part of the signed material, and the
// signature is checked before anything reaches the provider so an unverified
// caller cannot spend our outbound requests.
//
// Every refusal audits itself — deciding "this is not a dance we began" and
// deciding "that is worth a row" are one decision.
func (s *Service) CompleteDance(
	ctx context.Context,
	callback entity.DanceCallback,
	meta *entity.AuditMetadata,
) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.CompleteDance")
	defer span.End()

	// The provider reporting a failure of its own comes first. The user declined
	// or the provider stumbled; either way this dance is over and they start a
	// fresh one, which is one click on the same login page.
	if callback.ProviderError != "" {
		xlog.Warn(ctx, "oauth provider reported an error",
			xfield.String("provider_error", callback.ProviderError))
		s.publishLoginFailure(ctx, &entity.User{}, meta, entity.AuditFailureProviderDenied)

		return "", fmt.Errorf("%w: %s", apperr.ErrOAuthProviderDenied, callback.ProviderError)
	}

	provider, err := s.verifyDanceOrigin(ctx, callback, meta)
	if err != nil {
		return "", err
	}

	idToken, err := s.danceGateway.Exchange(ctx, callback.Code, callback.Verifier)
	if err != nil {
		xlog.Error(ctx, "oauth code exchange failed", xfield.Error(err))
		s.publishLoginFailure(ctx, &entity.User{}, meta, entity.AuditFailureProviderUnavailable)

		return "", fmt.Errorf("%w: %w", apperr.ErrOAuthExchangeFailed, err)
	}

	return s.issueDanceCode(ctx, provider, idToken, callback.InvitationHandle, meta)
}

// verifyDanceOrigin decides whether this callback belongs to a dance this
// backend began.
//
// All four refusals answer identically on purpose: with the state in a cookie
// an abandoned tab and a replayed URL are the same event, and telling a caller
// which half was wrong would confirm half a guess.
func (s *Service) verifyDanceOrigin(
	ctx context.Context,
	callback entity.DanceCallback,
	meta *entity.AuditMetadata,
) (entity.AuthMethod, error) {
	// Unaudited: a callback naming a provider we do not serve, or carrying no
	// state cookie at all, is a stranger knocking. /callback is unauthenticated
	// and reachable by anyone, so auditing that would fill the trail with rows
	// whose only content is an IP — and bury the rows that mean something.
	provider, ok := entity.DanceProvider(callback.Provider)
	if !ok {
		xlog.Warn(ctx, "oauth dance callback for an unsupported provider",
			xfield.String("provider", callback.Provider))

		return "", fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, callback.Provider)
	}

	// Verify would reject an empty signature on its own, so this looks
	// redundant — it is not. Falling through would take the audited branch
	// below, and "no cookie at all" is the one shape any scanner produces for
	// free. The check is here to keep it OUT of the trail, not to keep it out
	// of Verify.
	if callback.StateSignature == "" {
		xlog.Warn(ctx, "oauth dance callback arrived with no state cookie")

		return "", apperr.ErrOAuthDanceStateInvalid
	}

	// Audited from here on. A signature that is PRESENT and wrong is not a
	// stranger: either a real user whose cookie was mangled in transit — the
	// failure mode this design added, and one that exits as a 302 that reads as
	// success — or someone replaying a captured pair. Both are worth a row, and
	// a burst of them with no matching LoginSuccess is the signal that cookie
	// delivery has broken.
	if !s.danceSigner.Verify(callback.StateSignature, string(provider), callback.State, time.Now()) {
		return "", s.refuseDance(ctx, meta, "unverifiable state", apperr.ErrOAuthDanceStateInvalid)
	}

	// Past the signature the caller has proved the dance is ours, so detail
	// costs nothing — and a request that got this far and is still malformed is
	// odd enough to record.
	//
	// An absent verifier is refused; a merely stale one is NOT — nothing
	// enforces its lifetime server-side, so a stale one falls through and the
	// provider rejects the exchange, which is a provider failure rather than a
	// state one. An absent code is refused here too, saving an outbound request
	// guaranteed to fail.
	if callback.Verifier == "" {
		return "", s.refuseDance(ctx, meta, "no pkce verifier", apperr.ErrOAuthDanceStateInvalid)
	}

	if callback.Code == "" {
		return "", s.refuseDance(ctx, meta, "no authorization code", apperr.ErrOAuthDanceStateInvalid)
	}

	return provider, nil
}

// refuseDance records one refusal and returns the error to answer it with.
//
// One helper because these refusals ARE one answer: they share a reason, an
// error and a redirect code, and only the log line differs. Keeping them
// separate invited the reader to think the distinctions reached the caller,
// which is exactly what must not happen — telling a caller which half of its
// attempt was wrong confirms half a guess.
//
// The audit row is the only record that anything happened: the browser gets a
// 302, which reads as success in every access log.
func (s *Service) refuseDance(
	ctx context.Context,
	meta *entity.AuditMetadata,
	why string,
	err error,
) error {
	xlog.Warn(ctx, "oauth dance callback refused", xfield.String("reason", why))
	s.publishLoginFailure(ctx, &entity.User{}, meta, entity.AuditFailureSessionMismatch)

	return err
}

// issueDanceCode verifies the provider's id_token, resolves the user, mints the
// token pair and parks it behind a one-time code — the code CompleteDance hands
// back for the frontend to redeem later through RedeemDanceCode.
//
// Named for what it produces rather than for the step it sits in: an earlier
// name, redeemDance, read as the opposite of what it does and sat one screen
// above RedeemDanceCode, which genuinely redeems.
func (s *Service) issueDanceCode(
	ctx context.Context,
	provider entity.AuthMethod,
	idToken string,
	invitationHandle string,
	meta *entity.AuditMetadata,
) (string, error) {
	// The SAME verifier the BFF path uses: a second one would be a second place
	// for the audience and issuer checks to drift.
	claims, err := s.verifyProviderIDToken(ctx, provider, idToken)
	if err != nil {
		s.publishLoginFailure(ctx, &entity.User{}, meta, entity.AuditFailureProviderUnavailable)

		return "", fmt.Errorf("%w: %w", apperr.ErrOAuthExchangeFailed, err)
	}

	// PHASE 0, before anything is written: resolve the invitation and enforce
	// the email match. A refusal here means no account is created and no session
	// issued, which is the whole reason it precedes sign-in.
	invitation, err := s.resolveInvitation(ctx, invitationHandle, claims, meta)
	if err != nil {
		return "", err
	}

	// An invitation is what authorizes creating this account. Without one the
	// policy stays empty and an unknown user on an invite-only instance is
	// refused, exactly as before.
	policy := entity.UserCreationPolicy{AllowCreate: invitation != nil}

	// PHASE 1: create (or find) the user and issue the pair, in its own
	// transaction — GetOrCreateByAuthInfo's unique-violation recovery retries on
	// a clean connection and cannot be nested.
	pair, user, err := s.SignInWithVerifiedClaims(ctx, provider, claims, policy, meta)
	if err != nil {
		return "", fmt.Errorf("sign in with verified claims: %w", err)
	}

	// PHASE 2: spend the invitation and grant its roles, atomically.
	//
	// It runs after issuance because roles attach to a user id that did not
	// exist until phase 1. A failure here leaves a live session whose user holds
	// only the default roles while the invitation stays pending and its link
	// stays usable — the same direction the id_token accept path chose, and the
	// safer one: the alternative burns an invitation for a session nobody got.
	if invitation != nil {
		if err := s.invitations.ClaimForUser(ctx, invitation, user.ID); err != nil {
			return "", fmt.Errorf("claim invitation: %w", err)
		}
	}

	code, err := newDanceSecret()
	if err != nil {
		return "", fmt.Errorf("mint one-time dance code: %w", err)
	}

	// Minting and storing stay in one function: the code is worthless without
	// the entry and the entry unreachable without the code, so a caller able to
	// do one without the other could only get it wrong.
	if err := s.danceCodes.PutCode(ctx, code, pair); err != nil {
		return "", fmt.Errorf("store one-time dance code: %w", err)
	}

	return code, nil
}

// resolveInvitation runs phase 0 for an invited dance, returning nil for an
// ordinary one.
//
// Refusals are audited here rather than through refuseDance, which hard-codes
// AuditFailureSessionMismatch: filing "the invitation did not apply" as "the
// session nonce did not match" would make the trail state something untrue, and
// the trail is the only record a 302-terminated refusal leaves.
//
// The error is returned wrapped so errors.Is survives to danceFailureCode,
// which is what turns an email mismatch into a redirect the person can act on.
func (s *Service) resolveInvitation(
	ctx context.Context,
	handle string,
	claims *entity.OAuthIDTokenClaims,
	meta *entity.AuditMetadata,
) (*entity.ResolvedInvitation, error) {
	if handle == "" || s.invitations == nil {
		return nil, nil //nolint:nilnil // "not an invited dance" is the ordinary case, not an error.
	}

	invitation, err := s.invitations.ResolveForIdentity(ctx, handle, claims)
	if err != nil {
		xlog.Warn(ctx, "invited dance refused", xfield.Error(err))
		s.publishLoginFailure(ctx, &entity.User{Email: claims.Email, Name: claims.Name},
			meta, entity.AuditFailureInvitationRefused)

		return nil, fmt.Errorf("resolve invitation: %w", err)
	}

	return invitation, nil
}
