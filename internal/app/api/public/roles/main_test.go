package roles

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	redisDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/roles/models"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
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

	return New(services.User)
}

func createUser(ctx context.Context, t *testing.T, impl *Implementation, apiRoles ...apimodels.Role) *entity.User {
	t.Helper()

	user, err := impl.userSrv.GetOrCreateByOAuthInfo(ctx, &entity.OAuthProviderUserInfo{
		ID:    "oauth-" + uuid.NewString(),
		Email: uuid.NewString() + "@test.local",
		Name:  "Test User",
	})
	require.NoError(t, err)

	if len(apiRoles) == 0 {
		return user
	}

	roles := make([]entity.Role, 0, len(apiRoles))
	for _, role := range apiRoles {
		r, err := apimodels.FromAPIRole(role)
		require.NoError(t, err)
		roles = append(roles, r)
	}

	err = impl.userSrv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
		UserID: user.ID,
		Roles:  roles,
		Actor: &entity.User{
			ID:    user.ID,
			Email: user.Email,
			Roles: []entity.Role{entity.RoleAdmin},
		},
	})
	require.NoError(t, err)

	return user
}
