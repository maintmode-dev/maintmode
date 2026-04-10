package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"

	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var (
	db        *sqlx.DB
	userStore *users.Store
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	xlog.ReplaceGlobalLogger(xlog.NewZapAdapter(logger))

	conn := testdbconnutils.NewDB()
	db = conn
	closer.Add(conn.Close)

	userStore = users.NewStore(db)

	code := m.Run()

	closer.CloseAll(ctx)

	os.Exit(code)
}

func makeRefreshToken(ctx context.Context, t *testing.T, store *refreshtoken.Store) *entity.RefreshToken {
	t.Helper()

	user := testdbutils.MakeUser(ctx, t, userStore)

	refreshToken := &entity.RefreshToken{
		Token:      uuid.NewString(),
		UserID:     user.ID,
		Family:     uuid.New(),
		ExpiresAt:  xtime.UTCNow().Add(time.Hour),
		GraceTTL:   nil,
		Revoked:    false,
		ReplacedBy: nil,
		BoundIP:    "BoundIP",
	}

	err := store.Save(ctx, refreshToken)
	require.NoError(t, err)

	return refreshToken
}
