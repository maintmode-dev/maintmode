package user

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/license"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// breakGlassEmail returns an address in the reserved break-glass domain, one per
// test: the suite shares a database, so each test stands in for an instance of
// its own rather than sharing the one break-glass account.
func breakGlassEmail() string {
	return xuuid.NewString() + "@maintmode.invalid"
}

// The account is created on the first sign-in as an admin, the grant is
// audited once, and a repeat sign-in reaches the same account without a second
// record -- a trail of promotions that never happened stops being evidence.
func TestEnsureBreakGlassAccount_CreatesOneAuditedAdmin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	publisher := &recordingPublisher{}
	srv := NewService(
		dbtx.NewTxManager(db),
		users.NewStore(db),
		useridentities.NewStore(db),
		publisher,
		&fakeTokenRevoker{},
		license.NewNoop(),
		false,
		loginProviders,
	)
	email := breakGlassEmail()

	account, err := srv.EnsureBreakGlassAccount(ctx, email, "Break-glass admin")
	require.NoError(t, err)
	require.Equal(t, email, account.Email)
	require.Contains(t, account.Roles, entity.RoleAdmin)

	granted := publisher.rolesChanged()
	require.Len(t, granted, 1, "the admin grant must be recorded exactly once")
	require.Equal(t, entity.SystemUser, granted[0].Actor)
	require.Equal(t, account.ID, granted[0].Target.ID)

	again, err := srv.EnsureBreakGlassAccount(ctx, email, "Break-glass admin")
	require.NoError(t, err)
	require.Equal(t, account.ID, again.ID)
	require.Len(t, publisher.rolesChanged(), 1, "a repeat sign-in must not fabricate a second promotion")
}

// Break-glass is the way in when the admins are unreachable: an account someone
// demoted gets the admin role back on its next sign-in rather than signing in
// as a guest.
func TestEnsureBreakGlassAccount_RestoresTheAdminRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initService(t)
	email := breakGlassEmail()
	account, err := srv.EnsureBreakGlassAccount(ctx, email, "Break-glass admin")
	require.NoError(t, err)

	account.Roles = entity.DefaultRoles
	require.NoError(t, users.NewStore(db).Update(ctx, account))

	again, err := srv.EnsureBreakGlassAccount(ctx, email, "Break-glass admin")
	require.NoError(t, err)
	require.Contains(t, again.Roles, entity.RoleAdmin)
}

// A credential that stops working when the license runs out of seats fails
// exactly when the existing admins are unreachable and the org is at its cap.
func TestEnsureBreakGlassAccount_IgnoresTheSeatCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initServiceWithSeatGuard(t, &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded})

	account, err := srv.EnsureBreakGlassAccount(ctx, breakGlassEmail(), "Break-glass admin")
	require.NoError(t, err, "the break-glass admin must be creatable even at the seat cap")
	require.Contains(t, account.Roles, entity.RoleAdmin)
}

// Blocking the account is how an admin switches break-glass off; a sign-in must
// not undo it.
func TestEnsureBreakGlassAccount_LeavesABlockedAccountBlocked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initService(t)
	email := breakGlassEmail()
	account, err := srv.EnsureBreakGlassAccount(ctx, email, "Break-glass admin")
	require.NoError(t, err)
	require.NoError(t, srv.BlockUser(ctx, &entity.BlockUserCmd{
		Actor:  makeUser(ctx, t, srv, entity.RoleAdmin),
		UserID: account.ID,
	}))

	again, err := srv.EnsureBreakGlassAccount(ctx, email, "Break-glass admin")
	require.NoError(t, err)
	require.True(t, again.IsBlocked())
}

// The break-glass exemption must not widen: every other path stays capped.
// Without this the carve-out could be implemented by weakening the guard
// globally and nothing would notice.
func TestGetOrCreateByAuthInfo_NonBootstrapStaysSeatCapped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	guard := &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded}
	srv := initServiceWithSeatGuard(t, guard)

	// A Google login granted an admin role must still hit the cap.
	_, err := srv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "@email.com",
		Name:  "Capped User",
	}, entity.UserCreationPolicy{AllowCreate: true, GrantRoles: []entity.Role{entity.RoleAdmin}})
	require.ErrorIs(t, err, apperr.ErrSeatsLimitExceeded)

	// And so must an ordinary role grant.
	setupSrv := initService(t)
	target := makeUser(ctx, t, setupSrv)
	actor := makeUser(ctx, t, setupSrv, entity.RoleAdmin)

	_, err = srv.AssignRoles(ctx, &entity.AssignRolesCmd{
		Actor:  actor,
		UserID: target.ID,
		Roles:  []entity.Role{entity.RoleEditor},
	})
	require.ErrorIs(t, err, apperr.ErrSeatsLimitExceeded)
	require.Positive(t, guard.callCount(), "the guard must still fire for non-break-glass grants")
}

// recordingPublisher captures published actions instead of enqueuing them. The
// package's default publisher writes to the real outbox, which nothing drains
// in tests, so there is no way to read back what was recorded.
type recordingPublisher struct {
	mu        sync.Mutex
	published []audit.Action
}

func (p *recordingPublisher) Publish(_ context.Context, action audit.Action) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.published = append(p.published, action)

	return nil
}

func (p *recordingPublisher) rolesChanged() []audit.RolesChanged {
	p.mu.Lock()
	defer p.mu.Unlock()

	var out []audit.RolesChanged
	for _, a := range p.published {
		if rc, ok := a.(audit.RolesChanged); ok {
			out = append(out, rc)
		}
	}
	return out
}
