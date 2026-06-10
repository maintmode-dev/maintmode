package user

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/maintmode/internal/storages/audit"

	"github.com/ruko1202/maintmode/internal/services/auditor"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var db *sqlx.DB

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAuthAppConfig())
	closer.Add(db.Close)

	code := m.Run()

	os.Exit(code)
}

// fakeTokenRevoker records the user IDs whose tokens were revoked, so block
// tests can assert that BlockUser triggers session revocation.
type fakeTokenRevoker struct {
	revoked []uuid.UUID
}

func (f *fakeTokenRevoker) RevokeRefreshTokenByUserID(_ context.Context, userID uuid.UUID) error {
	f.revoked = append(f.revoked, userID)
	return nil
}

func initService(t *testing.T) *Service {
	t.Helper()
	srv, _ := initServiceWithRevoker(t)
	return srv
}

func initServiceWithRevoker(t *testing.T) (*Service, *fakeTokenRevoker) {
	t.Helper()

	revoker := &fakeTokenRevoker{}
	srv := NewService(
		config.DevEnvironment,
		dbtx.NewTxManager(db),
		users.NewStore(db),
		useridentities.NewStore(db),
		auditor.NewAuditor(audit.NewStore(db)),
		revoker,
	)
	return srv, revoker
}

// fixedAdminCountStore wraps the real users store but reports a fixed number of
// active admins. It lets last-admin lockout tests be deterministic on the shared
// dev DB (which carries many admins): every other method still hits the real DB,
// only CountActiveAdmins is forced.
type fixedAdminCountStore struct {
	UsersStore
	activeAdmins int64
}

func (s fixedAdminCountStore) CountActiveAdmins(context.Context) (int64, error) {
	return s.activeAdmins, nil
}

// initServiceWithAdminCount builds a service whose CountActiveAdmins always
// returns activeAdmins, so the last-admin guard can be exercised independently
// of the shared DB's real admin population.
func initServiceWithAdminCount(t *testing.T, activeAdmins int64) *Service {
	t.Helper()

	store := fixedAdminCountStore{UsersStore: users.NewStore(db), activeAdmins: activeAdmins}
	return NewService(
		config.DevEnvironment,
		dbtx.NewTxManager(db),
		store,
		useridentities.NewStore(db),
		auditor.NewAuditor(audit.NewStore(db)),
		&fakeTokenRevoker{},
	)
}

func makeUser(ctx context.Context, t *testing.T, srv *Service, roles ...entity.Role) *entity.User {
	t.Helper()

	created, err := srv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "@email.com",
		Name:  "Name" + t.Name(),
	})
	require.NoError(t, err)

	if uniqRoles := lo.Uniq(roles); len(uniqRoles) > 0 {
		err = srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  entity.SystemUser,
			UserID: created.ID,
			Roles:  uniqRoles,
		})
		require.NoError(t, err)
	}

	user, err := srv.GetByID(ctx, created.ID)
	require.NoError(t, err)

	return user
}
