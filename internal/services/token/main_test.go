package token

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	valkeylib "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"

	"github.com/ruko1202/maintmode/internal/storages/blacklisttoken"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/entity"
)

var (
	db     *sqlx.DB
	valkey *valkeylib.Client
)

const tokenTTL = 15 * time.Minute

func TestMain(m *testing.M) {
	cfg := config.LoadAppConfig()
	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	valkey = testdbconnutils.NewValkeyClient(cfg)
	closer.Add(valkey.Close)

	code := m.Run()

	os.Exit(code)
}

func initService(t *testing.T) *Service {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return NewService(
		dbtx.NewTxManager(db),
		refreshtoken.NewStore(db),
		blacklisttoken.NewStore(valkey),
		&config.JWT{AccessTokenTTL: tokenTTL, Issuer: "test-issuer", Kid: "kid-1"},
		key,
	)
}

func testUser(t *testing.T) *entity.User {
	t.Helper()

	return &entity.User{
		ID:    uuid.New(),
		Email: "alice@example.com",
		Roles: entity.DefaultRoles,
	}
}
