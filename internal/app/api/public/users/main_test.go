package users

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	redisDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db    *sqlx.DB
	redis *redisDB.Client
	cfg   *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadAuthConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	redis = testdbconnutils.NewRedisClient(cfg)
	closer.Add(redis.Close)

	code := m.Run()

	os.Exit(code)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	stores, err := bootstrap.NewStores(db, redis)
	require.NoError(t, err)

	services, err := bootstrap.NewServices(t.Context(), cfg, stores)
	require.NoError(t, err)

	return New(services.User)
}
