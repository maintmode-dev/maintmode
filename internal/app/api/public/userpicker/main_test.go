package userpicker

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	valkeyDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db     *sqlx.DB
	valkey *valkeyDB.Client
	cfg    *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	valkey = testdbconnutils.NewValkeyClient(cfg)
	closer.Add(valkey.Close)

	code := m.Run()

	os.Exit(code)
}

// newServices builds a per-test service set wired to the real auth services
// (user/auth in-process) backed by the live Postgres and Valkey.
func newServices(ctx context.Context, t *testing.T) *bootstrap.Services {
	t.Helper()

	return testbootstraputils.InitServicesT(ctx, t, db, valkey, cfg)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	services := newServices(context.Background(), t)
	return New(services.UserPicker)
}

// seedUserNamed provisions a real, persisted user with the given roles and
// messenger tags, and returns it. The picker reads through the real user
// service, so the rows a test asserts on have to actually exist.
//
// marker is embedded in the display name so the caller can scope a listing to
// exactly the users it seeded by passing the same marker as the search term —
// the shared database holds rows from every other test. label distinguishes
// several users seeded under one marker.
//
// Tags are written through the self-service preference path, which canonicalizes
// them exactly as production does; nil leaves the tag unset.
func seedUserNamed(
	ctx context.Context,
	t *testing.T,
	marker, label string,
	roles []entity.Role,
	telegramTag, slackTag *string,
) *entity.User {
	t.Helper()

	services := newServices(ctx, t)

	// Unique per run: `make tloc` runs with -count 2 against a shared database,
	// so identifiers derived from the test name would collide on the second pass.
	unique := uuid.NewString()

	user, err := services.User.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, &entity.OAuthProviderUserInfo{
		ID:    "picker-" + unique,
		Email: "picker-" + unique + "@test.local",
		Name:  "Picker " + label + " " + marker,
	}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	err = services.User.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
		Actor:  entity.SystemUser,
		UserID: user.ID,
		Roles:  roles,
	})
	require.NoError(t, err)
	user.Roles = roles

	if telegramTag != nil || slackTag != nil {
		updated, updErr := services.User.UpdatePreferences(ctx, user.ID, &entity.UpdatePreferencesCmd{
			SetTelegramTag: telegramTag != nil,
			TelegramTag:    telegramTag,
			SetSlackTag:    slackTag != nil,
			SlackTag:       slackTag,
		})
		require.NoError(t, updErr)
		require.NotNil(t, updated)
		user = updated
		user.Roles = roles
	}

	return user
}

// callerWithRoles builds an authenticated caller for the Echo context. The
// picker resolves the actor from there (mirroring what the auth middleware sets
// from the access token) to decide whether has_messenger_tag is meaningful.
func callerWithRoles(roles []entity.Role) *entity.User {
	return &entity.User{
		ID:    uuid.New(),
		Email: "caller-" + uuid.NewString() + "@test.local",
		Name:  "Caller",
		Roles: roles,
	}
}
