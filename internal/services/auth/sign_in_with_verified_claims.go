package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// SignInWithVerifiedClaims resolves an already-verified external identity to a
// user and mints a token pair for them.
//
// It takes CLAIMS rather than a token, and that is the whole point of it
// existing. Two paths reach this code: the BFF exchange, which verifies an
// id_token posted by the frontend, and the backend OAuth dance, which verifies
// the id_token it fetched from the provider itself. Passing a raw token here
// would force the dance to verify twice — and duplicating the body instead
// would let one identity resolve to two different users depending on which path
// a person took, which is precisely the bug the two paths running side by side
// would otherwise invite.
//
// It owns the whole audit trail for the sign-in — LoginFailed on both failure
// branches and LoginSuccess at the end — like every other service in this
// codebase. An earlier version left the success record to each caller on the
// theory that the two paths differed; they did not, and the two records were
// identical down to the SessionID.
func (s *Service) SignInWithVerifiedClaims(
	ctx context.Context,
	provider entity.AuthMethod,
	claims *entity.OAuthIDTokenClaims,
	policy entity.UserCreationPolicy,
	meta *entity.AuditMetadata,
) (*entity.TokenPair, *entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.SignInWithVerifiedClaims")
	defer span.End()

	user, err := s.usersSrv.GetOrCreateByAuthInfo(ctx, provider, &entity.OAuthProviderUserInfo{
		ID:    claims.Subject,
		Email: claims.Email,
		Name:  claims.Name,
	}, policy)
	if err != nil {
		// login_failed is a security-relevant record. It is published to the
		// durable audit outbox: the write survives a crash, at the cost of being
		// eventually-consistent rather than persisted before we return.
		s.publishLoginFailure(ctx, &entity.User{Email: claims.Email, Name: claims.Name},
			meta, provisioningFailureReason(err))

		return nil, nil, fmt.Errorf("get or create user: %w", err)
	}

	pair, err := s.IssueTokenPair(ctx, user, meta.IP)
	if err != nil {
		s.publishLoginFailure(ctx, user, meta, issuanceFailureReason(err))

		return nil, nil, fmt.Errorf("issue token pair: %w", err)
	}

	// Published here rather than by the caller: the SessionID that correlates a
	// login only exists once the pair is minted, and this is the first point
	// where both it and the user are in hand.
	success := *meta
	success.SessionID = pair.SessionID.String()

	s.publishAudit(ctx, audit.LoginSuccess{User: user, Meta: &success})

	return pair, user, nil
}
