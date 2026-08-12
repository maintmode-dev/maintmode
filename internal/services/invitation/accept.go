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

// Accept completes an invitation: it validates the token, verifies the OAuth
// payload, guards that the OAuth email matches the invited email, creates the
// user with the invitation's pre-assigned roles, marks the invitation accepted,
// and issues a backend token pair (like a normal login).
//
// Failure modes the caller must surface as a bare status string (no detail):
//   - apperr.ErrInvalidInvitation — token unknown/expired/accepted/revoked, or
//     the OAuth token does not verify ("invalid")
//   - apperr.ErrEmailMismatch — OAuth email != invited email ("email_mismatch")
//
// Ordering & transactions:
//   - GetOrCreateByOAuthInfo runs first, in its own transaction. Its
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

	provider, err := s.oauthProviders.Get(ctx, cmd.Provider)
	if err != nil {
		xlog.Error(ctx, "accept: get oauth provider failed", xfield.Error(err))
		return nil, apperr.ErrInvalidInvitation
	}

	claims, err := provider.VerifyToken(ctx, cmd.IDToken)
	if err != nil {
		xlog.Error(ctx, "accept: verify oauth token failed", xfield.Error(err))
		return nil, apperr.ErrInvalidInvitation
	}

	// Anti-takeover guard: the account being created must match the invited
	// email exactly (case-insensitive). No detail is leaked on mismatch.

	if !emailMatchesIgnoreCase(ctx, claims.Email, inv.Email) {
		xlog.Warn(ctx, "accept: oauth email does not match invitation")
		return nil, apperr.ErrEmailMismatch
	}

	// Resolve (or create) the user before the claim transaction.
	// GetOrCreateByOAuthInfo's concurrent-first-login recovery retries on a
	// clean connection after a unique violation aborts its own transaction, so
	// it must own its transaction and cannot be nested in the claim tx below.
	// The valid invitation itself authorizes the creation; the invitation's
	// roles are assigned by the claim transaction below, not via the policy.
	user, err := s.userSrv.GetOrCreateByOAuthInfo(ctx, cmd.Provider, &entity.OAuthProviderUserInfo{
		ID:    claims.Subject,
		Email: claims.Email,
		Name:  claims.Name,
	}, entity.UserCreationPolicy{AllowCreate: true})
	if err != nil {
		xlog.Error(ctx, "accept: get or create user failed", xfield.Error(err))
		return nil, fmt.Errorf("get or create user: %w", err)
	}

	// Claim the invitation and assign its roles atomically in one transaction.
	// The status-guarded MarkAccepted is the single-use gate: of two concurrent
	// accepts of the same token, exactly one flips pending→accepted
	// (claimed=true); the loser matches zero rows and is rejected as invalid.
	// Binding the claim and the role grant in a single tx means we never end up
	// with an accepted invitation whose user is missing roles — if AssignRoles
	// fails, the claim rolls back and the link stays usable. AssignRoles joins
	// this tx via the reentrant WithinTx.
	err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		claimed, err := s.store.MarkAccepted(ctx, inv.ID)
		if err != nil {
			return fmt.Errorf("mark accepted: %w", err)
		}
		if !claimed {
			return apperr.ErrInvalidInvitation
		}

		user, err = s.userSrv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  entity.SystemUser,
			UserID: user.ID,
			Roles:  inv.Roles,
		})
		if err != nil {
			return fmt.Errorf("assign invitation roles: %w", err)
		}
		return nil
	})
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
