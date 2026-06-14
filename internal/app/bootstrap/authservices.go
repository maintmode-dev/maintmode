package bootstrap

import (
	"context"
	"fmt"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/eventbus"
	auditorlistener "github.com/ruko1202/maintmode/internal/eventbus/listeners/auditor"
	"github.com/ruko1202/maintmode/internal/services/auditor"
	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/authz"
	"github.com/ruko1202/maintmode/internal/services/invitation"
	"github.com/ruko1202/maintmode/internal/services/messaging/scheduler"
	messagesender "github.com/ruko1202/maintmode/internal/services/messaging/sender"
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
	Invitation *invitation.Service
	Audit      *auditor.Auditor
	RBAC       *authz.CasbinAuthorizer
	StateCodec *statecodec.Service
	// Dispatcher owns the async listener goroutines; Stop it on shutdown so
	// in-flight audit writes drain before the process exits.
	Dispatcher *eventbus.Dispatcher
}

func NewAuthServices(
	ctx context.Context,
	cfg *config.AppConfig,
	stores *AuthStores,
	gateways *AuthGateways,
) (*AuthServices, error) {
	// Auditor выступает в двух ролях: read-сторона (api/public/audit читает логи
	// через AuthServices.Audit) и write-сторона (аудит-листенер диспетчера).
	auditorSrv := auditor.NewAuditor(
		stores.Audit,
	)
	dispatcher := eventbus.NewDispatcher(auditorlistener.NewListener(auditorSrv))

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
		dispatcher,
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

	oauthProviderList, err := initOAuthProviders(ctx, &cfg.OauthProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to init oauth providers: %w", err)
	}
	oauthProviders := oauthprovider.NewOAuthProviders(cfg, oauthProviderList)

	authSrv := auth.NewService(
		&cfg.JWT,
		stores.TxManager,
		userSrv,
		stores.Locker,
		stores.TokenBlackList,
		oauthProviders,
		tokenSrv,
		dispatcher,
	)

	return &AuthServices{
		Token: tokenSrv,
		User:  userSrv,
		Auth:  authSrv,
		Invitation: invitation.NewService(
			cfg,
			stores.TxManager,
			stores.UserInvitations,
			userSrv,
			authSrv,
			oauthProviders,
			messagesender.NewService(
				gateways.NotifyTransportRegistry,
				scheduler.NewService(goque.NewTaskQueueManager(stores.taskStorage)),
			),
		),
		Audit:      auditorSrv,
		RBAC:       authorizer,
		StateCodec: stateCodec,
		Dispatcher: dispatcher,
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
