package user

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/storages/audit"

	"github.com/ruko1202/maintmode/internal/services/auditor"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var db *sqlx.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	xlog.ReplaceGlobalLogger(xlog.NewZapAdapter(logger))

	db = testdbconnutils.NewDB()
	closer.Add(db.Close)

	code := m.Run()

	closer.CloseAll(ctx)

	os.Exit(code)
}

func initService(t *testing.T) *Service {
	t.Helper()

	return NewService(
		dbtx.NewTxManager(db),
		users.NewStore(db),
		auditor.NewAuditor(audit.NewStore(db)),
	)
}

func makeUser(ctx context.Context, t *testing.T, srv *Service, roles ...entity.Role) *entity.User {
	t.Helper()

	created, err := srv.GetOrCreateByOAuthInfo(ctx, &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "@email.com",
		Name:  "Name" + t.Name(),
	})
	require.NoError(t, err)

	if uniqRoles := lo.Uniq(roles); len(uniqRoles) > 0 {
		err = srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  &entity.User{},
			UserID: created.ID,
			Roles:  uniqRoles,
		})
		require.NoError(t, err)
	}

	user, err := srv.GetByID(ctx, created.ID)
	require.NoError(t, err)

	return user
}
