package user

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// recordingAuditPublisher captures published audit actions in memory so tests
// can assert on them. Thread-safe: the race tests publish from goroutines.
type recordingAuditPublisher struct {
	mu      sync.Mutex
	actions []audit.Action
}

func (p *recordingAuditPublisher) Publish(_ context.Context, action audit.Action) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.actions = append(p.actions, action)
	return nil
}

// rolesChanges returns the captured RolesChanged events.
func (p *recordingAuditPublisher) rolesChanges() []audit.RolesChanged {
	p.mu.Lock()
	defer p.mu.Unlock()

	var changes []audit.RolesChanged
	for _, action := range p.actions {
		if change, ok := action.(audit.RolesChanged); ok {
			changes = append(changes, change)
		}
	}
	return changes
}

// initPolicyService builds a service around the given users store (typically a
// count-forcing wrapper, so the bootstrap decision is deterministic on the
// shared DB) with a recording audit publisher.
func initPolicyService(t *testing.T, store UsersStore, allowOpenSignup bool) (*Service, *recordingAuditPublisher) {
	t.Helper()

	rec := &recordingAuditPublisher{}
	srv := NewService(
		dbtx.NewTxManager(db),
		store,
		useridentities.NewStore(db),
		rec,
		&fakeTokenRevoker{},
		allowOpenSignup,
	)
	return srv, rec
}

// oauthInfo builds a unique synthetic OAuth identity per call (shared DB,
// -count 2 runs).
func oauthInfo() *entity.OAuthProviderUserInfo {
	return &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "@policy-test.com",
		Name:  "Policy Test User",
	}
}

func TestGetOrCreateByOAuthInfo_CreationPolicy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// One active admin: the bootstrap branch stays off, only the policy decides.
	steadyStore := func() UsersStore {
		return fixedAdminCountStore{UsersStore: users.NewStore(db), activeAdmins: 1}
	}

	t.Run("deny: no invitation, signup closed, nothing persisted", func(t *testing.T) {
		t.Parallel()
		srv, _ := initPolicyService(t, steadyStore(), false)
		info := oauthInfo()

		user, err := srv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, info, entity.UserCreationPolicy{})
		require.ErrorIs(t, err, apperr.ErrSignupDisabled)
		require.Nil(t, user)

		// Zero rows behind: neither the user nor the identity exists.
		_, err = users.NewStore(db).GetByEmail(ctx, info.Email)
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
		_, err = useridentities.NewStore(db).GetByProviderSubject(ctx, entity.OAuthProviderGoogle, info.ID)
		require.ErrorIs(t, err, apperr.ErrProviderNotConnected)
	})

	t.Run("allow create: default roles", func(t *testing.T) {
		t.Parallel()
		srv, _ := initPolicyService(t, steadyStore(), false)

		user, err := srv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, oauthInfo(),
			entity.UserCreationPolicy{AllowCreate: true})
		require.NoError(t, err)
		require.Equal(t, entity.DefaultRoles, user.Roles)
	})

	t.Run("allow create with grant roles: union over defaults, audited", func(t *testing.T) {
		t.Parallel()
		srv, rec := initPolicyService(t, steadyStore(), false)

		user, err := srv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, oauthInfo(),
			entity.UserCreationPolicy{AllowCreate: true, GrantRoles: []entity.Role{entity.RoleEditor}})
		require.NoError(t, err)
		require.ElementsMatch(t, []entity.Role{entity.RoleGuest, entity.RoleEditor}, user.Roles)

		changes := rec.rolesChanges()
		require.Len(t, changes, 1)
		require.Equal(t, entity.SystemUser, changes[0].Actor)
		require.Equal(t, []entity.Role{entity.RoleEditor}, changes[0].Change.Roles)
	})

	t.Run("invalid grant role fails loudly, nothing persisted", func(t *testing.T) {
		t.Parallel()
		srv, _ := initPolicyService(t, steadyStore(), false)
		info := oauthInfo()

		user, err := srv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, info,
			entity.UserCreationPolicy{AllowCreate: true, GrantRoles: []entity.Role{"warlord"}})
		require.ErrorIs(t, err, apperr.ErrInvalidRole)
		require.Nil(t, user)

		_, err = users.NewStore(db).GetByEmail(ctx, info.Email)
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
	})

	t.Run("open signup: uninvited user becomes guest", func(t *testing.T) {
		t.Parallel()
		srv, _ := initPolicyService(t, steadyStore(), true)

		user, err := srv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, oauthInfo(), entity.UserCreationPolicy{})
		require.NoError(t, err)
		require.Equal(t, entity.DefaultRoles, user.Roles)
	})

	t.Run("existing user: login is a pure lookup, roles never change", func(t *testing.T) {
		t.Parallel()
		srv, rec := initPolicyService(t, steadyStore(), false)
		info := oauthInfo()

		created, err := srv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, info,
			entity.UserCreationPolicy{AllowCreate: true})
		require.NoError(t, err)
		require.Equal(t, entity.DefaultRoles, created.Roles)

		// A repeat login is a lookup: it returns the same user, and GrantRoles
		// on the policy does NOT escalate the roles of a user that already
		// exists. Logging in never grants privileges — that is an explicit
		// admin action, not a side effect of authentication.
		user, err := srv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, info,
			entity.UserCreationPolicy{AllowCreate: true, GrantRoles: []entity.Role{entity.RoleEditor}})
		require.NoError(t, err)
		require.Equal(t, created.ID, user.ID)
		require.Equal(t, entity.DefaultRoles, user.Roles, "repeat login must not add the granted role")

		// The login path published nothing — no roles changed, so no
		// RolesChanged audit event.
		require.Empty(t, rec.rolesChanges())
	})
}

func TestGetOrCreateByOAuthInfo_Bootstrap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Zero active admins reported on every count: the bootstrap branch is
	// always taken. The shared DB's real admins are invisible to the forced
	// count, so this exercises the empty-installation decision deterministically.
	emptyStore := func() UsersStore {
		return fixedAdminCountStore{UsersStore: users.NewStore(db), activeAdmins: 0}
	}

	t.Run("first login creates the admin and audits the grant", func(t *testing.T) {
		t.Parallel()
		srv, rec := initPolicyService(t, emptyStore(), false)

		user, err := srv.GetOrCreateByOAuthInfo(ctx, entity.OAuthProviderGoogle, oauthInfo(), entity.UserCreationPolicy{})
		require.NoError(t, err)
		require.True(t, user.IsAdmin(), "first user must be promoted to admin, got roles %v", user.Roles)

		// The bootstrap promotion is recorded by the plain RolesChanged event
		// that AssignRoles publishes — system actor, admin role.
		changes := rec.rolesChanges()
		require.Len(t, changes, 1)
		require.Equal(t, entity.SystemUser, changes[0].Actor)
		require.Equal(t, user.ID, changes[0].Target.ID)
		require.Equal(t, []entity.Role{entity.RoleAdmin}, changes[0].Change.Roles)
	})
}
