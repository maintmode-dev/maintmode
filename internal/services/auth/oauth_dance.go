package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// verifyProviderCredential turns the credential the gateway returned into the
// claims that identify a user.
//
// It routes through the SAME authmethod registry the BFF exchange uses, rather
// than reaching into a provider package directly, for two reasons: the audience
// and issuer checks then have exactly one implementation to drift from, and the
// dev/test `use_stub` substitution keeps applying to both paths at once. A dance
// that bypassed the registry would verify against real Google on a stand where
// every other login is stubbed.
//
// "Credential" rather than "id token" because what the string holds depends on
// the provider: an id_token for OIDC, an opaque access token for GitHub. The
// registry hands it to the provider that produced it, and only that provider
// knows how to read it.
func (s *Service) verifyProviderCredential(
	ctx context.Context,
	provider entity.AuthMethod,
	credential string,
) (*entity.OAuthIDTokenClaims, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.VerifyProviderCredential")
	defer span.End()

	authMethod, err := s.authMethods.Get(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("get oauth provider: %w", err)
	}

	claims, err := authMethod.Authenticate(ctx, credential)
	if err != nil {
		return nil, fmt.Errorf("verify provider credential: %w", err)
	}

	return claims, nil
}
