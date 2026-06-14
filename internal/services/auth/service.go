package auth

import (
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/eventbus"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/user"
	"github.com/ruko1202/maintmode/internal/storages/blacklisttoken"
	"github.com/ruko1202/maintmode/internal/storages/distributedlock"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

type Service struct {
	cfg            *config.JWT
	txManager      *dbtx.TxManager
	usersSrv       *user.Service
	tokenSrv       *token.Service
	oauthProviders *oauthprovider.Providers
	locker         *distributedlock.Store
	blacklistStore *blacklisttoken.Store
	dispatcher     *eventbus.Dispatcher
}

func NewService(
	cfg *config.JWT,
	txManager *dbtx.TxManager,
	usersSrv *user.Service,
	locker *distributedlock.Store,
	blacklistStore *blacklisttoken.Store,
	oauthProviders *oauthprovider.Providers,
	tokenSvc *token.Service,
	dispatcher *eventbus.Dispatcher,
) *Service {
	return &Service{
		cfg:            cfg,
		txManager:      txManager,
		usersSrv:       usersSrv,
		locker:         locker,
		blacklistStore: blacklistStore,
		oauthProviders: oauthProviders,
		tokenSrv:       tokenSvc,
		dispatcher:     dispatcher,
	}
}
