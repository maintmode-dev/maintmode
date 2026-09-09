package invitation

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// Accept completes an invitation from a provider id_token the CALLER supplies.
//
// Deprecated: use the invited OAuth dance instead — StartDance with an
// invitation token, resolved by ResolveForIdentity and spent by ClaimForUser.
// It does not require the caller to hold an id_token at all, which matters
// because the frontend stopped being an OAuth client and cannot produce one.
//
// It stays because the dance routes are registered only when the dance is
// configured: on an instance without it, the BFF exchange still yields an
// id_token and this is the only path an invited person has. Do not delete it
// until the dance is the only supported configuration.
//
// It validates the token, verifies the OAuth payload, guards that the OAuth
// email matches the invited email, creates the user with the invitation's
// pre-assigned roles, marks the invitation accepted, and issues a backend token
// pair (like a normal login).
//
// Failure modes the caller must surface as a bare status string (no detail):
//   - apperr.ErrInvalidInvitation — token unknown/expired/accepted/revoked, or
//     the OAuth token does not verify ("invalid")
//   - apperr.ErrEmailMismatch — OAuth email != invited email ("email_mismatch")
//
// Ordering & transactions:
//   - GetOrCreateByAuthInfo runs first, in its own transaction. Its
//     concurrent-first-login recovery retries on a clean connection after a
//     unique violation, so it must own its tx and cannot be nested.
//   - MarkAccepted (the single-use claim) and AssignRoles then run together in
//     one transaction. Binding them means an accepted invitation can never be
//     left with a user missing its roles: if role assignment fails, the claim
//     rolls back and the link stays usable.
//   - IssueTokenPair runs last as its own unit, like a normal login.
//
// Trade-off: because the user is created before the claim, two truly concurrent
// accepts of the same token can both reach user creation and race on the
// users.email unique constraint, so the loser may get a 500 rather than a clean
// "invalid". This window is tiny (same token, same instant) and is accepted in
// favor of the claim+roles atomicity above.
func (s *Service) Accept(ctx context.Context, cmd *entity.AcceptInvitationCmd) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.Accept")
	defer span.End()

	inv, err := s.store.GetByTokenHash(ctx, xhash.HashSha256([]byte(cmd.Token)))
	if err != nil {
		if errors.Is(err, apperr.ErrInvitationNotFound) {
			return nil, apperr.ErrInvalidInvitation
		}
		xlog.Error(ctx, "accept: lookup invitation failed", xfield.Error(err))
		return nil, err
	}

	// Any non-live status collapses to a single "invalid" — never leak which.
	if inv.EffectiveStatus(xtime.UTCNow()) != entity.InvitationStatusPending {
		xlog.Warn(ctx, "accept: invitation is not pending")
		return nil, apperr.ErrInvalidInvitation
	}

	// Parsed against the registry rather than a compiled-in list, so a provider
	// added by configuration works here without a code change. An unknown name
	// collapses into the same opaque refusal as every other failure below.
	method, ok := s.authMethods.Parse(cmd.Provider)
	if !ok {
		xlog.Warn(ctx, "accept: unknown provider named")
		return nil, apperr.ErrInvalidInvitation
	}

	provider, err := s.authMethods.Get(ctx, method)
	if err != nil {
		xlog.Error(ctx, "accept: get oauth provider failed", xfield.Error(err))
		return nil, apperr.ErrInvalidInvitation
	}

	claims, err := provider.Authenticate(ctx, cmd.IDToken)
	if err != nil {
		xlog.Error(ctx, "accept: verify oauth token failed", xfield.Error(err))
		return nil, apperr.ErrInvalidInvitation
	}

	// Anti-takeover guard: the account being created must match the invited
	// email exactly (case-insensitive). No detail is leaked on mismatch.
	//
	// The address is the credential on this path, so an address the issuer will
	// not vouch for is worth no more than one that does not match. An issuer
	// that lets a user self-assert any address would otherwise let that user
	// accept someone else's invitation.
	//
	// Checked here rather than left to the provider because this path does not
	// always run through an OIDC one: Methods.Get substitutes the stub for every
	// method on a use_stub stand, and the stub verifies nothing.
	//
	// Both refusals answer identically. A distinguishable "not verified" would
	// tell a token holder that the invited address MATCHED their own unverified
	// one, which is exactly what this file's no-detail contract hides.
	if !claims.EmailVerified {
		xlog.Warn(ctx, "accept: provider reports the email as unverified")
		return nil, apperr.ErrEmailMismatch
	}

	if !emailMatchesIgnoreCase(ctx, claims.Email, inv.Email) {
		xlog.Warn(ctx, "accept: oauth email does not match invitation")
		return nil, apperr.ErrEmailMismatch
	}

	// Resolve (or create) the user before the claim transaction.
	// GetOrCreateByAuthInfo's concurrent-first-login recovery retries on a
	// clean connection after a unique violation aborts its own transaction, so
	// it must own its transaction and cannot be nested in the claim tx below.
	// The valid invitation itself authorizes the creation; the invitation's
	// roles are assigned by the claim transaction below, not via the policy.
	user, err := s.userSrv.GetOrCreateByAuthInfo(ctx, method, &entity.OAuthProviderUserInfo{
		ID:    claims.Subject,
		Email: claims.Email,
		Name:  claims.Name,
	}, entity.UserCreationPolicy{AllowCreate: true})
	if err != nil {
		xlog.Error(ctx, "accept: get or create user failed", xfield.Error(err))
		return nil, fmt.Errorf("get or create user: %w", err)
	}

	// Claim the invitation and assign its roles atomically. Shared with the
	// invited dance's ClaimForUser: the single-use gate and the load-bearing
	// MarkAccepted-before-AssignRoles ordering are documented there, in the one
	// place both paths run them.
	user, err = s.claimAndAssignRoles(ctx, inv.ID, user.ID, inv.Roles)
	if err != nil {
		xlog.Error(ctx, "accept: claim and assign roles failed", xfield.Error(err))
		return nil, err
	}

	pair, err := s.tokenIssuer.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		xlog.Error(ctx, "accept: issue token pair failed", xfield.Error(err))
		return nil, fmt.Errorf("issue token pair: %w", err)
	}

	return pair, nil
}
