package bootstrap

import (
	"context"
	"fmt"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/auditor"
	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/authz"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider/googleoauth"
	statecodec "github.com/ruko1202/maintmode/internal/services/state_codec"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/user"
)

type AuthServices struct {
	Auth       *auth.Service
	Token      *token.Service
	User       *user.Service
	Audit      *auditor.Auditor
	RBAC       *authz.CasbinAuthorizer
	StateCodec *statecodec.Service
}

func NewAuthServices(
	ctx context.Context,
	cfg *config.AppConfig,
	stores *AuthStores,
) (*AuthServices, error) {
	auditorSrv := auditor.NewAuditor(
		stores.Audit,
	)

	// tokenSrv is built before userSrv: blocking a user revokes their refresh
	// tokens, so the user service depends on the token service.
	tokenSrv := token.NewService(
		stores.TxManager,
		stores.RefreshToken,
		cfg.JWT.GeneratePrivateKey(),
		cfg.JWT.Issuer,
		cfg.JWT.Kid,
	)

	userSrv := user.NewService(
		cfg.Environment,
		stores.TxManager,
		stores.Users,
		stores.UserIdentities,
		auditorSrv,
		tokenSrv,
	)

	authorizer, err := authz.NewCasbinAuthorizer(cfg.RBAC)
	if err != nil {
		return nil, fmt.Errorf("failed to init casbin authorizer: %w", err)
	}

	stateCodec := statecodec.NewService(
		[]byte(cfg.JWT.OAuthStateSigningKey),
		cfg.JWT.OAuthStateTTL,
	)

	oauthProviders, err := initOAuthProviders(ctx, &cfg.OauthProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to init oauth providers: %w", err)
	}

	return &AuthServices{
		Token: tokenSrv,
		User:  userSrv,
		Auth: auth.NewService(
			&cfg.JWT,
			stores.TxManager,
			userSrv,
			stores.Locker,
			stores.TokenBlackList,
			oauthprovider.NewOAuthProviders(cfg, oauthProviders),
			tokenSrv,
			auditorSrv,
		),
		Audit:      auditorSrv,
		RBAC:       authorizer,
		StateCodec: stateCodec,
	}, nil
}

func initOAuthProviders(ctx context.Context, cfg *config.OauthProviders) ([]oauthprovider.OAuthProvider, error) {
	google, err := googleoauth.NewProvider(ctx, &cfg.Google)
	if err != nil {
		return nil, fmt.Errorf("init google oauth provider: %w", err)
	}

	return []oauthprovider.OAuthProvider{
		google,
	}, nil
}
