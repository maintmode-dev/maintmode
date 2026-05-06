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
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"

	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/entity"
)

var db *sqlx.DB

const tokenTTL = 15 * time.Minute

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAuthAppConfig())
	closer.Add(db.Close)

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
		key,
		"test-issuer", "kid-1",
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
