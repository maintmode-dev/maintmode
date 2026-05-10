package bootstrap

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/auditor"
	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/authz"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
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
	cfg *config.AppConfig,
	stores *AuthStores,
) (*AuthServices, error) {
	auditorSrv := auditor.NewAuditor(
		stores.Audit,
	)

	userSrv := user.NewService(
		cfg.Environment,
		stores.TxManager,
		stores.Users,
		auditorSrv,
	)

	tokenSrv := token.NewService(
		stores.TxManager,
		stores.RefreshToken,
		cfg.JWT.GeneratePrivateKey(),
		cfg.JWT.Issuer,
		cfg.JWT.Kid,
	)

	authorizer, err := authz.NewCasbinAuthorizer(cfg.RBAC)
	if err != nil {
		return nil, fmt.Errorf("failed to init casbin authorizer: %w", err)
	}

	stateCodec := statecodec.NewService(
		[]byte(cfg.JWT.OAuthStateSigningKey),
		cfg.JWT.OAuthStateTTL,
	)

	return &AuthServices{
		Token: tokenSrv,
		User:  userSrv,
		Auth: auth.NewService(
			&cfg.JWT,
			stores.TxManager,
			userSrv,
			stores.Locker,
			stores.TokenBlackList,
			oauthprovider.NewOAuthProviders(cfg.Environment, &cfg.OauthProviders),
			tokenSrv,
			auditorSrv,
		),
		Audit:      auditorSrv,
		RBAC:       authorizer,
		StateCodec: stateCodec,
	}, nil
}
