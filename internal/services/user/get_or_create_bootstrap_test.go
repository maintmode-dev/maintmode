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

// bootstrapInfo builds the claims the bootstrap auth method produces: a
// constant subject (there is no upstream provider to issue one) and an email
// that comes from configuration.
//
// The subject is parameterised for tests only. In production it is the single
// constant entity.BootstrapSubject, which — with the global unique index on
// (provider, subject) — is exactly what limits an instance to ONE break-glass
// identity and makes a repeat login resolve the same user. These tests share
// one database and `make tloc` runs them twice, so a literal constant here
// would make every case after the first resolve the first case's user. Each
// test therefore gets its own subject, standing in for a separate instance.
func bootstrapInfo(subject, email string) *entity.OAuthProviderUserInfo {
	return &entity.OAuthProviderUserInfo{
		ID:    subject,
		Email: email,
		Name:  "Bootstrap Admin",
	}
}

// bootstrapIdentity returns a unique (subject, email) pair, one per test, for
// the reason given on bootstrapInfo.
func bootstrapIdentity() (subject, email string) {
	id := xuuid.NewString()
	return entity.BootstrapSubject + "-" + id, id + "@bootstrap.test"
}

// bootstrapPolicy mirrors what the auth service passes: creation is authorized
// by possession of the break-glass secret itself.
func bootstrapPolicy() entity.UserCreationPolicy {
	return entity.UserCreationPolicy{
		AllowCreate: true,
		GrantRoles:  []entity.Role{entity.RoleAdmin},
	}
}

// A break-glass login must work on an instance that already has admins — that
// is the whole point: it exists for when the existing admins are unreachable.
// Without AllowCreate the login would be refused with ErrSignupDisabled, since
// the zero-admin branch does not fire here.
func TestGetOrCreateByAuthInfo_BootstrapOnPopulatedInstance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initServiceWithAdminCount(t, 3) // admins already exist
	subject, email := bootstrapIdentity()

	user, err := srv.GetOrCreateByAuthInfo(
		ctx, entity.AuthMethodBootstrap, bootstrapInfo(subject, email), bootstrapPolicy(),
	)
	require.NoError(t, err)
	require.Equal(t, email, user.Email)
	require.Contains(t, user.Roles, entity.RoleAdmin)
}

// users.email is NOT NULL UNIQUE, so a bootstrap email that already belongs to
// someone (the ordinary case: an operator signs in via Google, Google breaks,
// they reach for break-glass) would hit a unique violation on create. The
// recovery in GetOrCreateByAuthInfo only catches ErrProviderAlreadyConnected —
// the identity index — not the users-email one, so break-glass would fail
// exactly when it is needed.
//
// Link instead of create: attach the bootstrap identity to the existing user.
func TestGetOrCreateByAuthInfo_BootstrapLinksToExistingUserByEmail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initServiceWithAdminCount(t, 3)
	existing := makeUser(ctx, t, srv) // a plain guest, created via the Google path
	subject, _ := bootstrapIdentity()

	user, err := srv.GetOrCreateByAuthInfo(
		ctx, entity.AuthMethodBootstrap, bootstrapInfo(subject, existing.Email), bootstrapPolicy(),
	)
	require.NoError(t, err)
	require.Equal(t, existing.ID, user.ID, "break-glass must let the operator back into their own account")
	require.Contains(t, user.Roles, entity.RoleAdmin, "the linked user must end up an admin")

	// And the identity is now resolvable, so the next login is a pure lookup.
	again, err := srv.GetOrCreateByAuthInfo(
		ctx, entity.AuthMethodBootstrap, bootstrapInfo(subject, existing.Email), bootstrapPolicy(),
	)
	require.NoError(t, err)
	require.Equal(t, existing.ID, again.ID)
}

// A repeat break-glass login resolves the same user rather than creating a
// second one — the constant subject is what guarantees it.
func TestGetOrCreateByAuthInfo_BootstrapIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initServiceWithAdminCount(t, 3)
	subject, email := bootstrapIdentity()

	first, err := srv.GetOrCreateByAuthInfo(
		ctx, entity.AuthMethodBootstrap, bootstrapInfo(subject, email), bootstrapPolicy(),
	)
	require.NoError(t, err)
	second, err := srv.GetOrCreateByAuthInfo(
		ctx, entity.AuthMethodBootstrap, bootstrapInfo(subject, email), bootstrapPolicy(),
	)
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
}

// A break-glass credential that stops working when the license runs out of
// seats is not a break-glass credential: it would fail exactly when the
// existing admins are unreachable AND the org is at its cap.
func TestGetOrCreateByAuthInfo_BootstrapIsExemptFromSeatCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	guard := &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded}
	srv := initServiceWithSeatGuard(t, guard)
	subject, email := bootstrapIdentity()

	user, err := srv.GetOrCreateByAuthInfo(
		ctx, entity.AuthMethodBootstrap, bootstrapInfo(subject, email), bootstrapPolicy(),
	)
	require.NoError(t, err, "break-glass must work with the seat cap exhausted")
	require.Contains(t, user.Roles, entity.RoleAdmin)
}

// The same exemption must cover the link branch. Scoping it to "creation only"
// would leave break-glass into an EXISTING user seat-capped, which is the same
// outage in a different shape.
func TestGetOrCreateByAuthInfo_BootstrapLinkIsExemptFromSeatCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Fixtures first, through a service whose guard never fires.
	setupSrv := initService(t)
	existing := makeUser(ctx, t, setupSrv)

	guard := &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded}
	srv := initServiceWithSeatGuard(t, guard)
	subject, _ := bootstrapIdentity()

	user, err := srv.GetOrCreateByAuthInfo(
		ctx, entity.AuthMethodBootstrap, bootstrapInfo(subject, existing.Email), bootstrapPolicy(),
	)
	require.NoError(t, err, "break-glass into an existing account must survive an exhausted cap")
	require.Equal(t, existing.ID, user.ID)
	require.Contains(t, user.Roles, entity.RoleAdmin)
}

// The exemption must not widen: every OTHER path stays capped. Without this the
// bootstrap carve-out could be implemented by weakening the guard globally and
// nothing would notice.
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
	require.Positive(t, guard.callCount(), "the guard must still fire for non-bootstrap grants")
}

// A break-glass login must never resurrect a blocked account.
//
// usersStore.Update is a blind full-column write: it persists Roles, BlockedAt
// and the profile fields from whatever struct it is handed. The link branch
// reads its user with a plain GetByEmail, so if that snapshot were written back
// it would restore blocked_at as it looked at read time.
//
// The race is narrow in wall-clock terms but the consequence is not: an admin
// blocking the account while a login is in flight would have the block silently
// undone by the login's own write, and IssueAccessToken — which checks the
// in-memory user — would hand out a token for a blocked account.
//
// blockOnReadStore makes the race deterministic: it blocks the user at the
// moment the login reads them, so the snapshot the branch carries is guaranteed
// stale. Prove-It — this fails when grantRolesUnguarded writes that snapshot
// instead of re-reading under FOR UPDATE.
type blockOnReadStore struct {
	UsersStore
	once   sync.Once
	onRead func()
}

func (s *blockOnReadStore) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	user, err := s.UsersStore.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	// Block AFTER the read returns, so the caller holds a pre-block snapshot.
	s.once.Do(s.onRead)
	return user, nil
}

func TestGetOrCreateByAuthInfo_BootstrapDoesNotUnblockAUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setupSrv := initService(t)
	existing := makeUser(ctx, t, setupSrv)

	racing := &blockOnReadStore{
		UsersStore: users.NewStore(db),
		onRead: func() {
			require.NoError(t, setupSrv.BlockUser(ctx, &entity.BlockUserCmd{
				Actor:  entity.SystemUser,
				UserID: existing.ID,
			}))
		},
	}

	srv := NewService(
		dbtx.NewTxManager(db),
		racing,
		useridentities.NewStore(db),
		newTestAuditPublisher(t),
		&fakeTokenRevoker{},
		license.NewNoop(),
		false,
	)

	subject, _ := bootstrapIdentity()
	user, err := srv.GetOrCreateByAuthInfo(
		ctx, entity.AuthMethodBootstrap, bootstrapInfo(subject, existing.Email), bootstrapPolicy(),
	)
	require.NoError(t, err)
	require.Equal(t, existing.ID, user.ID)

	require.NotNil(t, user.BlockedAt, "the login must not un-block the account in memory")

	persisted, err := setupSrv.GetByID(ctx, existing.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted.BlockedAt, "the login must not un-block the account in the database")
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

// The admin grant must leave an audit record. SPEC names the audit trail as one
// of the compensating controls that make a permanently-live break-glass
// endpoint acceptable, and this promotion is the single most security-relevant
// thing the endpoint does — an unrecorded admin grant would undercut the
// argument the whole design rests on.
//
// The no-op half matters just as much: a repeat break-glass login into an
// account that is ALREADY admin must publish nothing, or the trail fills with
// promotions that never happened and stops being readable evidence.
func TestGetOrCreateByAuthInfo_BootstrapPublishesRolesChanged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	publisher := &recordingPublisher{}
	srv := NewService(
		dbtx.NewTxManager(db),
		fixedAdminCountStore{UsersStore: users.NewStore(db), activeAdmins: 3},
		useridentities.NewStore(db),
		publisher,
		&fakeTokenRevoker{},
		license.NewNoop(),
		false,
	)

	subject, email := bootstrapIdentity()
	user, err := srv.GetOrCreateByAuthInfo(
		ctx, entity.AuthMethodBootstrap, bootstrapInfo(subject, email), bootstrapPolicy(),
	)
	require.NoError(t, err)
	require.Contains(t, user.Roles, entity.RoleAdmin)

	granted := publisher.rolesChanged()
	require.Len(t, granted, 1, "the admin grant must be recorded exactly once")
	require.Equal(t, entity.SystemUser, granted[0].Actor)
	require.Equal(t, user.ID, granted[0].Target.ID)
	require.Contains(t, granted[0].Change.Roles, entity.RoleAdmin)
	require.Equal(t, audit.RolesAssigned, granted[0].Kind)

	// A repeat login resolves the same user and changes nothing, so it must add
	// no second record.
	_, err = srv.GetOrCreateByAuthInfo(
		ctx, entity.AuthMethodBootstrap, bootstrapInfo(subject, email), bootstrapPolicy(),
	)
	require.NoError(t, err)
	require.Len(t, publisher.rolesChanged(), 1,
		"a repeat break-glass login must not fabricate a second promotion")
}
