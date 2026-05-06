package auth

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5/echotest"
	redisDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db    *sqlx.DB
	redis *redisDB.Client
	cfg   *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = config.LoadAuthAppConfig()
	cfg.OauthProviders.UseStub = true

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	redis = testdbconnutils.NewRedisClient(cfg)
	closer.Add(redis.Close)

	code := m.Run()

	os.Exit(code)
}

const authPolicy = `
g, editor, guest
g, reviewer, editor
g, admin, reviewer

p, guest, auth.roles.read, execute
p, guest, auth.user_roles.read, execute

p, admin, auth.roles.manage, execute
p, admin, auth.audit.read, execute
`

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	cfg.RBAC = config.RbacConfig{
		ModelPath:  "../../../../../deployment/auth/authz/model.conf",
		Adapter:    config.AuthorizationAdapterMemory,
		PolicyData: authPolicy,
	}
	services, err := bootstrap.NewAuthServices(cfg, bootstrap.NewAuthStores(db, redis))
	require.NoError(t, err)

	return New(
		services.Auth,
		services.Token,
		cfg.App.FrontendURL,
	)
}

func issueTokenPair(ctx context.Context, t *testing.T, impl *Implementation) *entity.TokenPair {
	t.Helper()

	c := echotest.ContextConfig{}.ToContext(t)

	tokenPair, err := impl.authSrv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
		Provider:     entity.OAuthProviderGoogle,
		CallbackCode: "string",
		ClientIP:     c.RealIP(),
	})
	require.NoError(t, err)

	return tokenPair
}
