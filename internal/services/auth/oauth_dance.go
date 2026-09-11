package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// verifyProviderIDToken verifies an id_token the backend fetched itself.
//
// It routes through the SAME authmethod registry the BFF exchange uses, rather
// than reaching into a provider package directly, for two reasons: the audience
// and issuer checks then have exactly one implementation to drift from, and the
// dev/test `use_stub` substitution keeps applying to both paths at once. A dance
// that bypassed the registry would verify against real Google on a stand where
// every other login is stubbed.

func (s *Service) verifyProviderIDToken(
	ctx context.Context,
	provider entity.AuthMethod,
	idToken string,
) (*entity.OAuthIDTokenClaims, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.VerifyProviderIDToken")
	defer span.End()

	authMethod, err := s.authMethods.Get(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("get oauth provider: %w", err)
	}

	claims, err := authMethod.Authenticate(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("verify provider id token: %w", err)
	}

	return claims, nil
}
