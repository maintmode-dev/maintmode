package testbootstraputils

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	redisDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// InitStores builds the store set against a real Postgres and Redis. Redis is
// required: the integration tests now run against the real auth services
// (user/auth in-process), and the active-token / locker / blacklist stores are
// Redis-backed.
func InitStores(
	db *sqlx.DB,
	redis *redisDB.Client,
) *bootstrap.Stores {
	stores, err := bootstrap.NewStores(db, redis)
	if err != nil {
		panic(err)
	}

	return stores
}

// InitServicesT builds the full service set wired to the REAL auth services
// (user/auth/token in-process) backed by the given Postgres and Redis. There are
// no auth mocks: the core services (maint approver check, userpicker,
// usersummary) resolve users through the real user service, and the active-token
// check runs against the real auth service.
func InitServicesT(
	ctx context.Context,
	t *testing.T,
	db *sqlx.DB,
	redis *redisDB.Client,
	cfg *config.AppConfig,
) *bootstrap.Services {
	t.Helper()

	services, err := bootstrap.NewServices(ctx, cfg, InitStores(db, redis))
	require.NoError(t, err)

	return services
}

// SeedEligibleApprover provisions a real, persisted, approver-eligible user via
// the user service and returns it. The returned user's ID is what tests pass as
// ApproverUserID so the maint create/update approver-eligibility check
// (reviewer/admin, not blocked) passes against the real user backend.
//
// It creates the user through GetOrCreateByOAuthInfo (synthetic OAuth identity)
// and then replaces its roles with RoleReviewer via ReplaceRoles (a SystemUser
// actor avoids the self-revoke guard), making it eligible regardless of
// environment-specific default-role assignment.
func SeedEligibleApprover(ctx context.Context, t *testing.T, services *bootstrap.Services) *entity.User {
	t.Helper()

	user, err := services.User.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, &entity.OAuthProviderUserInfo{
		ID:    "approver-" + uuid.NewString(),
		Email: "approver-" + uuid.NewString() + "@test.local",
		Name:  "Eligible Approver",
	})
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
