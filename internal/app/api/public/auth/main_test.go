package auth

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5/echotest"
	valkeyDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db     *sqlx.DB
	valkey *valkeyDB.Client
	cfg    *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadAuthConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	valkey = testdbconnutils.NewValkeyClient(cfg)
	closer.Add(valkey.Close)

	code := m.Run()

	os.Exit(code)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	stores, err := bootstrap.NewStores(db, valkey)
	require.NoError(t, err)

	services, err := bootstrap.NewServices(t.Context(), cfg, stores)
	require.NoError(t, err)

	return New(
		services.Auth,
		services.Token,
		services.User,
	)
}

func issueTokenPair(ctx context.Context, t *testing.T, impl *Implementation) *entity.TokenPair {
	t.Helper()

	c := echotest.ContextConfig{}.ToContext(t)

	tokenPair, err := impl.authSrv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
		Provider: entity.OAuthProviderGoogle,
		IDToken:  "stub-id-token",
		ClientIP: c.RealIP(),
	})
	require.NoError(t, err)

	return tokenPair
}
