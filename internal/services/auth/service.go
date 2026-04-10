package auth

import (
	"time"

	"github.com/ruko1202/maintmode/internal/services/auditor"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/user"
	"github.com/ruko1202/maintmode/internal/storages/blacklisttoken"
	"github.com/ruko1202/maintmode/internal/storages/distributedlock"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

const (
	distributedLockTTL = 5 * time.Second
	gracePeriod        = 30 * time.Second
	accessTokenTTL     = 15 * time.Minute
	refreshTokenTTL    = 30 * 24 * time.Hour
)

type Service struct {
	txManager      *dbtx.TxManager
	usersSrv       *user.Service
	tokenSrv       *token.Service
	oauthProviders *oauthprovider.Providers
	locker         *distributedlock.Store
	blacklistStore *blacklisttoken.Store
	auditorSrv     *auditor.Auditor
	getNowF        func() time.Time
}

func NewService(
	txManager *dbtx.TxManager,
	usersSrv *user.Service,
	locker *distributedlock.Store,
	blacklistStore *blacklisttoken.Store,
	oauthProviders *oauthprovider.Providers,
	tokenSvc *token.Service,
	auditorSrv *auditor.Auditor,
) *Service {
	return &Service{
		txManager:      txManager,
		usersSrv:       usersSrv,
		locker:         locker,
		blacklistStore: blacklistStore,
		oauthProviders: oauthProviders,
		tokenSrv:       tokenSvc,
		auditorSrv:     auditorSrv,
		getNowF:        xtime.UTCNow,
	}
}
