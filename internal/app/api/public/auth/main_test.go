package auth

import (
	"context"
	"os"
	"testing"
	"time"

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

	return newImpl(t, cfg.Auth)
}

// initImplWithOTPFloor builds the handler with an explicit response floor, so a
// test can assert the floor is applied without waiting out the production one.
func initImplWithOTPFloor(t *testing.T, floor time.Duration) *Implementation {
	t.Helper()

	return newImpl(t, config.Auth{OTPResponseFloor: floor})
}

func newImpl(t *testing.T, authCfg config.Auth) *Implementation {
	t.Helper()

	stores, err := bootstrap.NewStores(db, valkey)
	require.NoError(t, err)

	services, err := bootstrap.NewServices(t.Context(), cfg, stores)
	require.NoError(t, err)

	return New(authCfg, services.Auth, services.Token, services.User, services.OTP)
}

func issueTokenPair(ctx context.Context, t *testing.T, impl *Implementation) *entity.TokenPair {
	t.Helper()

	c := echotest.ContextConfig{}.ToContext(t)

	tokenPair, err := impl.authSrv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
		Provider: entity.AuthMethodGoogle,
		IDToken:  "stub-id-token",
		ClientIP: c.RealIP(),
	})
	require.NoError(t, err)

	return tokenPair
}
