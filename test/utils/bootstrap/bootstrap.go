package testbootstraputils

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	valkeyDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// InitStores builds the store set against a real Postgres and Valkey. Valkey is
// required: the integration tests now run against the real auth services
// (user/auth in-process), and the active-token / locker / blacklist stores are
// Valkey-backed.
func InitStores(
	db *sqlx.DB,
	valkey *valkeyDB.Client,
) *bootstrap.Stores {
	stores, err := bootstrap.NewStores(db, valkey)
	if err != nil {
		panic(err)
	}

	return stores
}

// InitServicesT builds the full service set wired to the REAL auth services
// (user/auth/token in-process) backed by the given Postgres and Valkey. There are
// no auth mocks: the core services (maint approver check, userpicker,
// usersummary) resolve users through the real user service, and the active-token
// check runs against the real auth service.
func InitServicesT(
	ctx context.Context,
	t *testing.T,
	db *sqlx.DB,
	valkey *valkeyDB.Client,
	cfg *config.AppConfig,
) *bootstrap.Services {
	t.Helper()

	SeedLoginProvidersT(ctx, t, db)

	services, err := bootstrap.NewServices(ctx, cfg, InitStores(db, valkey))
	require.NoError(t, err)

	return services
}

// SeedLoginProvidersT makes sure the registry holds the login providers these
// suites sign in through. Every suite that builds the real services must call
// it before a sign-in: identities reference a registry row, and on a fresh
// database -- CI, or a stack just recreated -- there is none, so a sign-in
// through `google` fails with "unsupported provider".
//
// A shared database hides this. Rows left by an earlier run make the suite pass
// locally while the same commit fails in CI.
func SeedLoginProvidersT(ctx context.Context, t *testing.T, db *sqlx.DB) {
	t.Helper()

	_, err := testdbutils.SeedLoginProviders(ctx, db, "services-suite-kek",
		entity.AuthMethodGoogle, entity.AuthMethodGithub)
	require.NoError(t, err)
}

// SeedEligibleApprover provisions a real, persisted, approver-eligible user via
// the user service and returns it. The returned user's ID is what tests pass as
// ApproverUserID so the maint create/update approver-eligibility check
// (reviewer/admin, not blocked) passes against the real user backend.
//
// It creates the user through GetOrCreateByAuthInfo (synthetic OAuth identity)
// and then replaces its roles with RoleReviewer via ReplaceRoles (a SystemUser
// actor avoids the self-revoke guard), making it eligible regardless of
// environment-specific default-role assignment.
func SeedEligibleApprover(ctx context.Context, t *testing.T, services *bootstrap.Services) *entity.User {
	t.Helper()

	user, err := services.User.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
		ID:    "approver-" + uuid.NewString(),
		Email: "approver-" + uuid.NewString() + "@test.local",
		Name:  "Eligible Approver",
	}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	err = services.User.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
		Actor:  entity.SystemUser,
		UserID: user.ID,
		Roles:  []entity.Role{entity.RoleReviewer},
	})
	require.NoError(t, err)

	user.Roles = []entity.Role{entity.RoleReviewer}

	return user
}
