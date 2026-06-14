package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	redisDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/services/oauthprovider"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/maintmode/internal/storages/audit"

	"github.com/ruko1202/maintmode/internal/eventbus"
	auditorlistener "github.com/ruko1202/maintmode/internal/eventbus/listeners/auditor"
	"github.com/ruko1202/maintmode/internal/services/auditor"

	mock_oauthprovider "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/oauthprovider"

	"github.com/ruko1202/maintmode/internal/services/user"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/storages/blacklisttoken"
	"github.com/ruko1202/maintmode/internal/storages/distributedlock"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db    *sqlx.DB
	redis *redisDB.Client
	cfg   *config.AppConfig
)

const (
	tokenIssuer = "test-issuer"
)

func TestMain(m *testing.M) {
	cfg = config.LoadAuthAppConfig()
	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	redis = testdbconnutils.NewRedisClient(cfg)
	closer.Add(redis.Close)

	code := m.Run()

	os.Exit(code)
}

type serviceMocks struct {
	oauthProvider *mock_oauthprovider.MockOAuthProvider
}

func initService(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mocks := &serviceMocks{
		oauthProvider: mock_oauthprovider.NewMockOAuthProvider(ctrl),
	}
	mocks.oauthProvider.EXPECT().
		ProviderID().
		Return(entity.OAuthProviderGoogle).
		AnyTimes()

	txManager := dbtx.NewTxManager(db)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tokenSrv := token.NewService(
		txManager,
		refreshtoken.NewStore(db),
		key,
		tokenIssuer,
		"kid-1",
	)

	return NewService(
		&cfg.JWT,
		txManager,
		user.NewService(
			config.DevEnvironment,
			txManager,
			users.NewStore(db),
			useridentities.NewStore(db),
			eventbus.NewDispatcher(auditorlistener.NewListener(auditor.NewAuditor(audit.NewStore(db)))),
			tokenSrv,
		),
		distributedlock.NewStore(redis),
		blacklisttoken.NewStore(redis),
		oauthprovider.NewOAuthProviders(cfg, []oauthprovider.OAuthProvider{mocks.oauthProvider}),
		tokenSrv,
		eventbus.NewDispatcher(auditorlistener.NewListener(auditor.NewAuditor(audit.NewStore(db)))),
	), mocks
}

func handleCallbackMock(mocks *serviceMocks, times int) *entity.OAuthProviderUserInfo {
	oauthUserToken := &entity.OAuthProviderTokens{AccessToken: "access-token"}
	mocks.oauthProvider.EXPECT().
		Exchange(gomock.Any(), "code-1").
		Return(oauthUserToken, nil).
		Times(times)

	oauthUser := &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "_alice@example.com",
		Name:  "alice",
	}
	mocks.oauthProvider.EXPECT().
		UserInfo(gomock.Any(), oauthUserToken.AccessToken).
		Return(oauthUser, nil).
		Times(times)

	return oauthUser
}
