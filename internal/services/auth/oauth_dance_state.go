package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	providerSegment string,
	invitationToken string,
	linkTicket string,
	binding string,
) (*entity.DanceStart, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.StartDance",
		xfield.String("provider", providerSegment))
	defer span.End()

	// The allow-list runs FIRST, before any secret is minted or stored: an
	// unknown provider must cost nothing.
	provider, ok := s.authMethods.DanceProvider(providerSegment)
	if !ok {
		return nil, fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, providerSegment)
	}

	// A dance without a browser binding could never be redeemed, so it is
	// refused before the person is sent through the provider for nothing.
	if !validDanceBinding(binding) {
		return nil, fmt.Errorf("%w: missing or malformed browser binding", apperr.ErrOAuthDanceStateInvalid)
	}

	// Both at once is refused rather than resolved in favor of one. Link mode
	// skips the invitation phases entirely, so accepting the pair would mint and
	// then silently drop a single-use invitation handle -- burning an invitation
	// the person could still have used.
	if linkTicket != "" && invitationToken != "" {
		return nil, fmt.Errorf("%w: an invitation cannot be combined with a link", apperr.ErrValidation)
	}

	// Validated BEFORE anything is minted, for the same reason the provider
	// allow-list runs first: a refusal must cost nothing. Read, not consumed --
	// the spend belongs at the callback, where the link actually happens.
	if err := s.verifyLinkTicket(ctx, linkTicket, provider); err != nil {
		return nil, err
	}

	state, err := newDanceSecret()
	if err != nil {
		return nil, fmt.Errorf("mint oauth state: %w", err)
	}

	verifier, err := newDanceSecret()
	if err != nil {
		return nil, fmt.Errorf("mint pkce verifier: %w", err)
	}

	gateway, err := s.danceGatewayFor(provider)
	if err != nil {
		return nil, err
	}

	invitationHandle, err := s.mintInvitationHandle(ctx, invitationToken)
	if err != nil {
		return nil, err
	}

	// Built before the struct rather than inline: the endpoint comes from the
	// provider's discovery document, so an instance whose IdP has not answered
	// yet fails here instead of handing the browser a URL with no host.
	authorizationURL, err := gateway.AuthCodeURL(ctx, state, verifier)
	if err != nil {
		return nil, fmt.Errorf("build authorization url: %w", err)
	}

	return &entity.DanceStart{
		State:            state,
		StateSignature:   s.danceSigner.Sign(string(provider), state, time.Now().Add(s.danceStateTTL)),
		Verifier:         verifier,
		AuthorizationURL: authorizationURL,
		TTL:              s.danceStateTTL,
		InvitationHandle: invitationHandle,
		LinkTicket:       linkTicket,
		Binding:          binding,
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
	// No token: an ordinary dance.
	if invitationToken == "" {
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

// RedeemDanceCode trades a one-time opaque code for the pair parked behind it,
// provided proof is the nonce the dance was bound to. A nil pair with no error
// means nothing to redeem -- unknown, expired, spent or bound to another
// browser, deliberately indistinguishable. The consume is atomic, so N
// concurrent redemptions yield one winner, and a wrong proof spends the code
// like a right one: a code gets one guess.
func (s *Service) RedeemDanceCode(ctx context.Context, code, proof string) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.RedeemDanceCode")
	defer span.End()

	entry, err := s.danceCodes.ConsumeCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("consume one-time dance code: %w", err)
	}
	if entry == nil {
		return nil, nil //nolint:nilnil // nothing to redeem; see the doc comment
	}

	if !danceBindingProven(entry.Binding, proof) {
		xlog.Warn(ctx, "one-time dance code presented without its browser binding")

		return nil, nil //nolint:nilnil // a code bound elsewhere redeems nothing; see the doc comment
	}

	return entry.Pair, nil
}

// oauthErrorAccessDenied is the RFC 6749 §4.1.2.1 error code a provider
// redirects back with when the resource owner denies the request -- in
// practice, the person canceling the consent screen.
const oauthErrorAccessDenied = "access_denied"

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
) (*entity.DanceOutcome, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.CompleteDance")
	defer span.End()

	// The provider reporting a failure of its own comes first. The user declined
	// or the provider stumbled; either way this dance is over and they start a
	// fresh one, which is one click on the same login page.
	if callback.ProviderError != "" {
		xlog.Warn(ctx, "oauth provider reported an error",
			xfield.String("provider_error", callback.ProviderError))
		s.publishLoginFailure(ctx, &entity.User{}, meta, entity.AuditFailureProviderDenied)

		// Matched exactly: the error parameter is a registered code, and a
		// substring match would let "access_denied_by_policy" -- a provider
		// refusing, not the person -- read as the person changing their mind.
		if callback.ProviderError == oauthErrorAccessDenied {
			return nil, apperr.ErrOAuthConsentDeclined
		}

		return nil, fmt.Errorf("%w: %s", apperr.ErrOAuthProviderDenied, callback.ProviderError)
	}

	provider, err := s.verifyDanceOrigin(ctx, callback, meta)
	if err != nil {
		return nil, err
	}

	gateway, err := s.danceGatewayFor(provider)
	if err != nil {
		xlog.Error(ctx, "no gateway for dance provider", xfield.Error(err))

		return nil, err
	}

	credential, err := gateway.Exchange(ctx, callback.Code, callback.Verifier)
	if err != nil {
		xlog.Error(ctx, "oauth code exchange failed", xfield.Error(err))
		s.publishLoginFailure(ctx, &entity.User{}, meta, entity.AuditFailureProviderUnavailable)

		return nil, fmt.Errorf("%w: %w", apperr.ErrOAuthExchangeFailed, err)
	}

	return s.issueDanceCode(ctx, provider, credential, callback, meta)
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
	provider, ok := s.authMethods.DanceProvider(callback.Provider)
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

	// /start refuses a dance without one, so an absent binding here is a cookie
	// lost in transit or a hand-built request. Either way the code it would get
	// could not be redeemed, so the provider is not asked for it.
	if callback.Binding == "" {
		return "", s.refuseDance(ctx, meta, "no browser binding", apperr.ErrOAuthDanceStateInvalid)
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
	credential string,
	callback entity.DanceCallback,
	meta *entity.AuditMetadata,
) (*entity.DanceOutcome, error) {
	// The SAME verifier the BFF path uses: a second one would be a second place
	// for the audience and issuer checks to drift.
	claims, err := s.verifyProviderCredential(ctx, provider, credential)
	if err != nil {
		s.publishLoginFailure(ctx, &entity.User{}, meta, credentialFailureReason(err))

		// The wrap is UNCHANGED, and deliberately so. Every failure here keeps
		// reaching danceFailureCode's ErrOAuthExchangeFailed arm and keeps
		// answering the browser with provider_error. A dedicated redirect code
		// for the account-side failures would be unreachable -- that arm matches
		// first -- so the distinction lives in the audit reason above, where
		// something can actually act on it.
		return nil, fmt.Errorf("%w: %w", apperr.ErrOAuthExchangeFailed, err)
	}

	// The LINK branch sits here: after the provider has vouched for the identity,
	// and BEFORE phase 0. Everything below -- the invitation resolution, user
	// creation, the one-time code -- belongs to signing in, and a link runs none
	// of it. Redeeming later would create a user and grant invitation roles
	// before discovering the dance was a link at all.
	//
	// The PRESENCE of the ticket is the discriminator, not what redeeming it
	// yields. Once a browser has presented one, the person asked to link, and no
	// redeem result may turn that back into a sign-in -- a spent or unknown
	// ticket is a refusal, not an absent one.
	if callback.LinkTicket != "" {
		return s.parkLink(ctx, provider, callback.LinkTicket, claims, callback.Binding, meta)
	}

	// PHASE 0, before anything is written: resolve the invitation and enforce
	// the email match. A refusal here means no account is created and no session
	// issued, which is the whole reason it precedes sign-in.
	invitation, err := s.resolveInvitation(ctx, callback.InvitationHandle, claims, meta)
	if err != nil {
		return nil, err
	}

	// An invitation is what authorizes creating this account. Without one the
	// policy stays empty and an unknown user on an invite-only instance is
	// refused, exactly as before.
	policy := entity.UserCreationPolicy{AllowCreate: invitation != nil}

	// PHASE 1: create (or find) the user, in its own transaction —
	// GetOrCreateByAuthInfo's unique-violation recovery retries on a clean
	// connection and cannot be nested.
	user, err := s.resolveSignInUser(ctx, provider, claims, policy, meta)
	if err != nil {
		return nil, fmt.Errorf("sign in with verified claims: %w", err)
	}

	// PHASE 2: spend the invitation and grant its roles, atomically.
	//
	// BEFORE issuance, like the id_token accept path: the access token carries
	// the roles the user holds when it is minted, so a pair issued first would
	// hand an invited admin a guest token until its next refresh. A failure here
	// issues no session and leaves the invitation pending, so following the link
	// again finishes the job. A failure at issuance below leaves the invitation
	// spent, but the account then exists with its roles and an ordinary sign-in
	// reaches it.
	if invitation != nil {
		// The invitation was matched against the identity's email, but the user
		// was found by the identity's SUBJECT. They differ when this identity is
		// linked to an account under another address -- and that link may have
		// been planted -- so the invitation's roles would land on an account the
		// invitee never was. Refused rather than granted.
		if !strings.EqualFold(user.Email, claims.Email) {
			xlog.Warn(ctx, "invited identity is linked to an account under another email")
			s.publishLoginFailure(ctx, user, meta, entity.AuditFailureInvitationRefused)

			return nil, fmt.Errorf("claim invitation: %w", apperr.ErrEmailMismatch)
		}

		claimed, claimErr := s.invitations.ClaimForUser(ctx, invitation, user.ID)
		if claimErr != nil {
			s.publishLoginFailure(ctx, user, meta, entity.AuditFailureUserProvisioning)

			return nil, fmt.Errorf("claim invitation: %w", claimErr)
		}
		user = claimed
	}

	// PHASE 3: issue the pair for the user as it now stands.
	pair, err := s.issueSignInPair(ctx, user, meta)
	if err != nil {
		return nil, fmt.Errorf("sign in with verified claims: %w", err)
	}

	code, err := newDanceSecret()
	if err != nil {
		return nil, fmt.Errorf("mint one-time dance code: %w", err)
	}

	// Minting and storing stay in one function: the code is worthless without
	// the entry and the entry unreachable without the code, so a caller able to
	// do one without the other could only get it wrong.
	if err := s.danceCodes.PutCode(ctx, code, entity.DanceCode{Pair: pair, Binding: callback.Binding}); err != nil {
		return nil, fmt.Errorf("store one-time dance code: %w", err)
	}

	return &entity.DanceOutcome{Code: code}, nil
}

// credentialFailureReason decides which incident a failed credential
// verification was.
//
// The two GitHub sentinels mean the provider answered correctly about an account
// this backend cannot accept: nothing is broken, and the person fixes it on
// GitHub. Everything else -- a refused token, an unreachable API, an id_token
// that will not verify -- means something IS broken, and an operator should go
// looking.
//
// Filing the first kind as the second is what would send that operator chasing a
// fault that does not exist, on a signal the trail is the only source of.
func credentialFailureReason(err error) entity.AuditFailureReason {
	if errors.Is(err, apperr.ErrGithubEmailUnusable) || errors.Is(err, apperr.ErrGithubIdentityUnusable) {
		return entity.AuditFailureProviderRejected
	}

	return entity.AuditFailureProviderUnavailable
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
	if handle == "" {
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
