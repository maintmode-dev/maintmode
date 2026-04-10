package bootstrap

import (
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/auditor"
	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/user"
)

type AuthServices struct {
	Auth  *auth.Service
	Token *token.Service
	User  *user.Service
	Audit *auditor.Auditor
}

func NewAuthServices(
	cfg *config.AppConfig,
	stores *AuthStores,
) *AuthServices {
	auditorSrv := auditor.NewAuditor(
		stores.Audit,
	)

	userSrv := user.NewService(
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

	return &AuthServices{
		Token: tokenSrv,
		User:  userSrv,
		Auth: auth.NewService(
			stores.TxManager,
			userSrv,
			stores.Locker,
			stores.TokenBlackList,
			oauthprovider.NewOAuthProviders(cfg.Environment, &cfg.OauthProviders),
			tokenSrv,
			auditorSrv,
		),
		Audit: auditorSrv,
	}
}
