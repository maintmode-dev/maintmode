package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	valkeyDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/services/oauthprovider"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/services/auditpublisher"

	mock_oauthprovider "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/oauthprovider"

	"github.com/ruko1202/maintmode/internal/services/user"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/license"
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
	db     *sqlx.DB
	valkey *valkeyDB.Client
	cfg    *config.AppConfig
)

const (
	tokenIssuer = "test-issuer"
)

func TestMain(m *testing.M) {
	cfg = config.LoadAppConfig()
	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	valkey = testdbconnutils.NewValkeyClient(cfg)
	closer.Add(valkey.Close)

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

	// Each service gets its own JWT config copy: parallel subtests tweak fields
	// like RefreshTokenGracePeriod, and sharing the package-level cfg.JWT pointer
	// would race under -race.
	jwtCfg := cfg.JWT

	return NewService(
		&jwtCfg,
		txManager,
		user.NewService(
			txManager,
			users.NewStore(db),
			useridentities.NewStore(db),
			newTestAuditPublisher(t),
			tokenSrv,
			license.NewNoop(), // auth tests do not exercise the seat cap
			// allowOpenSignup mirrors the local/dev config: a plain exchange of an
			// unknown user provisions a guest, so login tests need no invitation.
			true,
		),
		distributedlock.NewStore(valkey),
		blacklisttoken.NewStore(valkey),
		oauthprovider.NewOAuthProviders(cfg, []oauthprovider.OAuthProvider{mocks.oauthProvider}),
		tokenSrv,
		newTestAuditPublisher(t),
	), mocks
}

// newTestAuditPublisher builds the audit publisher backed by the test DB's goque
// queue. These tests exercise auth flows, not the audit drain, so events enqueue
// durably and nothing processes them — the tests here don't assert on audit_log
// rows.
func newTestAuditPublisher(t *testing.T) *auditpublisher.Publisher {
	t.Helper()
	storage, err := goque.NewStorage(db)
	require.NoError(t, err)
	return auditpublisher.New(goque.NewTaskQueueManager(storage))
}

// exchangeIDTokenMock sets up the VerifyToken expectation for the ID-token
// exchange flow and returns the identity the mocked provider resolves. Tests use
// it to mint a token pair via srv.ExchangeIDToken. The returned value carries
// the resolved email so callers can assert on the provisioned user.
func exchangeIDTokenMock(mocks *serviceMocks, times int) *entity.OAuthProviderUserInfo {
	oauthUser := &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "_alice@example.com",
		Name:  "alice",
	}
	mocks.oauthProvider.EXPECT().
		VerifyToken(gomock.Any(), "id-token").
		Return(&entity.OAuthIDTokenClaims{
			Subject: oauthUser.ID,
			Email:   oauthUser.Email,
			Name:    oauthUser.Name,
		}, nil).
		Times(times)

	return oauthUser
}
