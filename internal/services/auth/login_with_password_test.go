package auth

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xcripto"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// recordingAuditPublisher captures published actions instead of enqueuing them,
// so a test can assert what was recorded. The default test publisher writes to
// the real outbox, where nothing drains it and nothing can be read back.
type recordingAuditPublisher struct {
	mu        sync.Mutex
	published []audit.Action
}

func newRecordingAuditPublisher() *recordingAuditPublisher {
	return &recordingAuditPublisher{}
}

func (p *recordingAuditPublisher) Publish(_ context.Context, action audit.Action) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.published = append(p.published, action)

	return nil
}

func (p *recordingAuditPublisher) actions() []audit.Action {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]audit.Action(nil), p.published...)
}

// Why several subtests below are NOT parallel:
//
// entity.BootstrapSubject is a per-instance constant, so every break-glass
// login in this package resolves to the SAME user and shares one seed denylist.
// Running those concurrently lets one subtest's Retire spend another's seed.
// Subtests that never touch a seed stay parallel.

// bootstrapAddress returns an address unique to one test. Production has one
// break-glass admin per instance; these tests share a database, so each stands
// in for a separate instance.
func bootstrapAddress() string {
	return xuuid.NewString() + "@bootstrap.test"
}

func TestLoginWithPassword(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("issues an admin token pair", func(t *testing.T) {
		// NOT parallel: one shared bootstrap admin, see the note above.

		email := bootstrapAddress()
		password := "the-break-glass-" + xuuid.NewString()
		srv, _ := initServiceWithBootstrap(t, email, password)

		pair, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email:    email,
			Password: password,
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)
		require.NotEmpty(t, pair.AccessToken)
		require.NotEmpty(t, pair.RefreshToken)

		access, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.Contains(t, access.UserRoles, entity.RoleAdmin,
			"the break-glass admin must actually be an admin")

		// The identity is NOT asserted here. entity.BootstrapSubject is a
		// per-instance constant, so on this shared test database every
		// break-glass login resolves to whichever admin was provisioned first.
		// That is correct in production, where one instance has one break-glass
		// admin, and it makes the address a property of the database rather
		// than of this test.
	})

	// The address now selects the method: a break-glass password submitted
	// against some other address must not sign anyone in. Before RUK-289 the
	// body email was ignored entirely and any address reached this provider.
	t.Run("the break-glass password only answers for its own address", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithBootstrap(t, bootstrapAddress(), "the-break-glass-"+xuuid.NewString())

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email:    "someone-else@example.com",
			Password: "the-break-glass-password",
			ClientIP: "10.0.0.1",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	})

	// The break-glass address must not be locatable by timing. Every failing
	// branch of this endpoint spends one argon2id verification; the wrong-seed
	// branch is a constant-time compare over a config string and would answer
	// microseconds instead of tens of milliseconds without the decoy.
	t.Run("a wrong break-glass password costs the same as any other failure", func(t *testing.T) {
		t.Parallel()

		email := bootstrapAddress()
		srv, _ := initServiceWithBootstrap(t, email, "the-break-glass-"+xuuid.NewString())

		measure := func(address string) time.Duration {
			start := time.Now()
			_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
				Email: address, Password: "definitely-not-the-password", ClientIP: "10.0.0.1",
			})
			require.Error(t, err)

			return time.Since(start)
		}

		atBootstrap := measure(email)
		elsewhere := measure("someone-else@example.com")

		// Both pay one argon2id. Asserting a ratio rather than a difference
		// keeps this meaningful on slower machines; the gap being closed is
		// three orders of magnitude, so a 4x band cannot hide it.
		require.Less(t, atBootstrap, elsewhere*4,
			"the break-glass address must not answer measurably faster than any other")
		require.Less(t, elsewhere, atBootstrap*4,
			"nor measurably slower")
	})

	t.Run("a wrong password yields no token", func(t *testing.T) {
		t.Parallel()

		email := bootstrapAddress()
		srv, _ := initServiceWithBootstrap(t, email, "the-break-glass-"+xuuid.NewString())

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email:    email,
			Password: "not-the-password",
			ClientIP: "10.0.0.1",
		})
		require.Error(t, err)
	})

	// Step 1 of the resolution order beats step 2: once a stored password exists
	// for an address, the break-glass credential is not consulted for it.
	t.Run("a stored password takes precedence over the break-glass one", func(t *testing.T) {
		t.Parallel()

		seedPassword := "the-break-glass-" + xuuid.NewString()
		user := makePasswordUser(ctx, t, nil, "")

		// The break-glass provider answers for this user's own address, so both
		// credentials are candidates for the same login.
		srv, _ := initServiceWithBootstrap(t, user.Email, seedPassword)

		own, err := xcripto.HashPassword("my-own-personal-password")
		require.NoError(t, err)
		require.NoError(t, srv.passwords.UpsertPassword(ctx, user.ID, own))

		_, err = srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: "my-own-personal-password", ClientIP: "10.0.0.1",
		})
		require.NoError(t, err, "the personal password must work")

		_, err = srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: seedPassword, ClientIP: "10.0.0.1",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials,
			"the break-glass password must not be consulted once a personal one exists")
	})

	// An ordinary user with a stored password signs in through the same route.
	t.Run("an ordinary user signs in with their own password", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)

		user := makePasswordUser(ctx, t, srv, "an-ordinary-password")

		pair, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: "an-ordinary-password", ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)
		require.NotEmpty(t, pair.AccessToken)

		_, err = srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: "the-wrong-one", ClientIP: "10.0.0.1",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	})

	// A sha256 digest in the password column is a bug in whatever wrote it, and
	// must fail the login rather than read as a mismatch.
	t.Run("a credential written by the wrong hash function is refused", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)

		user := makePasswordUser(ctx, t, srv, "an-ordinary-password")
		require.NoError(t, srv.passwords.UpsertPassword(ctx, user.ID,
			"5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8"))

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: "an-ordinary-password", ClientIP: "10.0.0.1",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	})
}

// makePasswordUser provisions an ordinary user. When srv is non-nil the user is
// given the supplied password; passing nil provisions the row only, for tests
// that install the credential themselves.
func makePasswordUser(ctx context.Context, t *testing.T, srv *Service, password string) *entity.User {
	t.Helper()

	provisioner := srv
	if provisioner == nil {
		provisioner, _ = initServiceForMethod(t, entity.AuthMethodGoogle)
	}

	user, err := provisioner.usersSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle,
		&entity.OAuthProviderUserInfo{
			ID:    xuuid.NewString(),
			Email: xuuid.NewString() + "@example.com",
			Name:  "ordinary user",
		}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	if srv != nil {
		hash, hashErr := xcripto.HashPassword(password)
		require.NoError(t, hashErr)
		require.NoError(t, srv.passwords.UpsertPassword(ctx, user.ID, hash))
	}

	return user
}

func TestLoginWithPassword_Audit(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("a success is audited", func(t *testing.T) {
		// NOT parallel: see the seed subtests above -- one shared bootstrap user.

		email := bootstrapAddress()
		password := "the-break-glass-" + xuuid.NewString()
		srv, _ := initServiceWithBootstrap(t, email, password)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: email, Password: password, ClientIP: "10.0.0.1", UserAgent: "curl/8",
		})
		require.NoError(t, err)

		actions := publisher.actions()
		require.NotEmpty(t, actions)
		success, ok := actions[len(actions)-1].(audit.LoginSuccess)
		require.True(t, ok, "the last event must be a login success, got %T", actions[len(actions)-1])
		require.Equal(t, "10.0.0.1", success.Meta.IP)
		require.Equal(t, "curl/8", success.Meta.UserAgent)
	})

	// A wrong password is the event this endpoint most needs recorded, and it is
	// NOT inherited from the OAuth path: there a failing Authenticate returns
	// before any publish. It also has no resolved user, so the record carries a
	// synthetic one — the renderer dereferences the actor unconditionally.
	t.Run("a wrong password is audited with its own reason", func(t *testing.T) {
		t.Parallel()

		email := bootstrapAddress()
		srv, _ := initServiceWithBootstrap(t, email, "the-break-glass-"+xuuid.NewString())
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: email, Password: "wrong", ClientIP: "10.0.0.2",
		})
		require.Error(t, err)

		actions := publisher.actions()
		require.Len(t, actions, 1)
		failed, ok := actions[0].(audit.LoginFailed)
		require.True(t, ok, "expected a login failure, got %T", actions[0])
		require.NotNil(t, failed.User, "a nil user panics the audit renderer")
		require.Equal(t, entity.AuditFailureInvalidCredentials, failed.Meta.FailureReason)
		require.Equal(t, "10.0.0.2", failed.Meta.IP)

		// Drive it through the real renderer: a synthetic user with a zero ID is
		// the documented shape for a pre-identification failure, but only the
		// renderer can prove it does not panic on one.
		payload, renderErr := audit.NewRenderer().Render(failed)
		require.NoError(t, renderErr)
		require.Equal(t, entity.AuditActionLoginFailed, payload.Action)
	})
}

// TestLoginWithPassword_AuditNamesTheMethod pins which credential answered.
//
// The break-glass password is permanently live, and the argument for leaving it
// that way rather than demoting it to a one-time seed is that every use is on
// the record. That argument only holds if the record says which credential
// answered, so these assertions are the argument itself, not decoration.
func TestLoginWithPassword_AuditNamesTheMethod(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("a break-glass success is labeled bootstrap", func(t *testing.T) {
		// NOT parallel: one shared bootstrap user.

		email := bootstrapAddress()
		password := "the-break-glass-" + xuuid.NewString()
		srv, _ := initServiceWithBootstrap(t, email, password)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: email, Password: password, ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		actions := publisher.actions()
		require.NotEmpty(t, actions)
		success, ok := actions[len(actions)-1].(audit.LoginSuccess)
		require.True(t, ok, "expected a login success, got %T", actions[len(actions)-1])
		require.Equal(t, entity.AuditLoginMethodBootstrap, success.Meta.LoginMethod)
	})

	// The mutation guard for the case above: a constant wired in one place
	// satisfies the bootstrap assertion and fails here.
	t.Run("a stored-password success is labeled password", func(t *testing.T) {
		t.Parallel()

		password := "her-own-password-" + xuuid.NewString()
		srv, _ := initServiceWithBootstrap(t, bootstrapAddress(), "unrelated-"+xuuid.NewString())
		user := makePasswordUser(ctx, t, srv, password)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: password, ClientIP: "10.0.0.3",
		})
		require.NoError(t, err)

		actions := publisher.actions()
		require.NotEmpty(t, actions)
		success, ok := actions[len(actions)-1].(audit.LoginSuccess)
		require.True(t, ok, "expected a login success, got %T", actions[len(actions)-1])
		require.Equal(t, entity.AuditLoginMethodPassword, success.Meta.LoginMethod)
	})

	// THE SECURITY ASSERTION.
	//
	// A wrong password submitted against the break-glass address must not be
	// labeled. Labeling it would let anyone who can read the audit log sort
	// failed sign-ins by method and learn which address the break-glass
	// credential answers for -- on an instance where it has never been used
	// successfully, that is the only thing keeping the address out of the trail.
	// It would disclose by record precisely what the decoy hash on that path
	// spends an argon2id verification to hide from timing.
	//
	// A single method threaded uniformly through the failure publishes satisfies
	// both success assertions above and fails here.
	t.Run("a wrong break-glass password is not labeled", func(t *testing.T) {
		t.Parallel()

		email := bootstrapAddress()
		srv, _ := initServiceWithBootstrap(t, email, "the-break-glass-"+xuuid.NewString())
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: email, Password: "wrong", ClientIP: "10.0.0.4",
		})
		require.Error(t, err)

		actions := publisher.actions()
		require.Len(t, actions, 1)
		failed, ok := actions[0].(audit.LoginFailed)
		require.True(t, ok, "expected a login failure, got %T", actions[0])
		require.Empty(t, failed.Meta.LoginMethod,
			"labeling this branch turns the audit log into an oracle for the break-glass address")
	})

	// The other half of the address gate: a wrong address is equally unlabeled,
	// so the two cannot be told apart by method either.
	t.Run("a wrong address is not labeled", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithBootstrap(t, bootstrapAddress(), "the-break-glass-"+xuuid.NewString())
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: "nobody-" + xuuid.NewString() + "@example.com", Password: "wrong", ClientIP: "10.0.0.5",
		})
		require.Error(t, err)

		actions := publisher.actions()
		require.Len(t, actions, 1)
		failed, ok := actions[0].(audit.LoginFailed)
		require.True(t, ok, "expected a login failure, got %T", actions[0])
		require.Empty(t, failed.Meta.LoginMethod)
	})

	// Issuance refused after a correct stored password: the mirror of the
	// break-glass branch below, and labeled for the same reason.
	t.Run("issuance refused after a correct stored password is labeled password", func(t *testing.T) {
		t.Parallel()

		password := "her-own-password-" + xuuid.NewString()
		srv, _ := initServiceWithBootstrap(t, bootstrapAddress(), "unrelated-"+xuuid.NewString())
		user := makePasswordUser(ctx, t, srv, password)

		// Blocking the user is what makes IssueTokenPair refuse: the guard lives
		// inside IssueAccessToken, so this is the same path an ordinary blocked
		// user takes. The actor is required -- BlockUser has no nil-safe
		// degradation, by design.
		actor := makePasswordUser(ctx, t, nil, "")
		require.NoError(t, srv.usersSrv.BlockUser(ctx, &entity.BlockUserCmd{
			UserID: user.ID,
			Actor:  actor,
		}))

		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: password, ClientIP: "10.0.0.8",
		})
		require.Error(t, err)

		actions := publisher.actions()
		require.Len(t, actions, 1)
		failed, ok := actions[0].(audit.LoginFailed)
		require.True(t, ok, "expected a login failure, got %T", actions[0])
		require.Equal(t, entity.AuditFailureTokenIssuance, failed.Meta.FailureReason)
		require.Equal(t, entity.AuditLoginMethodPassword, failed.Meta.LoginMethod)
	})

	// A user whose own password is wrong IS labeled: the rule is "a credential
	// verified or was tried against a credential this user actually has", not
	// "the address matched". Nothing is disclosed -- the address is the user's
	// own, and the trail already carries it as the actor.
	t.Run("a wrong stored password is labeled password", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithBootstrap(t, bootstrapAddress(), "unrelated-"+xuuid.NewString())
		user := makePasswordUser(ctx, t, srv, "her-own-password-"+xuuid.NewString())
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: "wrong", ClientIP: "10.0.0.6",
		})
		require.Error(t, err)

		actions := publisher.actions()
		require.Len(t, actions, 1)
		failed, ok := actions[0].(audit.LoginFailed)
		require.True(t, ok, "expected a login failure, got %T", actions[0])
		require.Equal(t, entity.AuditLoginMethodPassword, failed.Meta.LoginMethod)
	})
}

// TestLoginWithPassword_UnconfiguredBreakGlassIsIndistinguishable covers the
// instance that has no break-glass at all.
//
// Once an empty configured value stopped conjuring a password, "no break-glass"
// became a state an instance can actually be in -- and the refusal it produces
// must look exactly like a refusal against a wrong address. Otherwise the trail
// or the clock tells an attacker which instances have an emergency entrance and
// which do not, and on the ones that do, which addresses have no stored
// password.
func TestLoginWithPassword_UnconfiguredBreakGlassIsIndistinguishable(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("the refusal is audited like any other", func(t *testing.T) {
		t.Parallel()

		email := bootstrapAddress()
		srv, _ := initServiceWithBootstrap(t, email, "")
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		// The empty password specifically, not just any wrong one: an
		// unconfigured instance resolves to an empty credential, so submitting
		// the same empty string is the shape that would make it a skeleton key
		// if the guard in Authenticate ever went away.
		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: email, Password: "", ClientIP: "10.0.0.20",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials,
			"an unconfigured break-glass must refuse like a wrong credential, "+
				"not like an unsupported provider")

		actions := publisher.actions()
		require.Len(t, actions, 1,
			"a refusal that publishes nothing means the path returned early, "+
				"before the audit record and before the decoy")
		failed, ok := actions[0].(audit.LoginFailed)
		require.True(t, ok, "expected a login failure, got %T", actions[0])
		require.Equal(t, entity.AuditFailureInvalidCredentials, failed.Meta.FailureReason)
		require.Empty(t, failed.Meta.LoginMethod,
			"nothing verified, so nothing is named")
	})

	// The timing half. Measured ACROSS configurations because that is the only
	// pair this change can break: within one unconfigured service both branches
	// already burn the decoy on lines nothing here touches, so such a comparison
	// is green before and after and proves nothing.
	t.Run("an unconfigured refusal costs the same as a configured one", func(t *testing.T) {
		t.Parallel()

		measure := func(srv *Service, address string) time.Duration {
			start := time.Now()
			_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
				Email: address, Password: "definitely-not-the-password", ClientIP: "10.0.0.21",
			})
			require.Error(t, err)

			return time.Since(start)
		}

		unconfiguredEmail := bootstrapAddress()
		unconfigured, _ := initServiceWithBootstrap(t, unconfiguredEmail, "")

		configuredEmail := bootstrapAddress()
		configured, _ := initServiceWithBootstrap(t, configuredEmail,
			"the-break-glass-"+xuuid.NewString())

		// Sequential, in one subtest: two services on two parallel subtests would
		// add scheduling noise to a comparison that does not need it. One
		// discarded pass each first, so neither side is charged for lazily
		// initialized state the other has already paid for.
		measure(unconfigured, unconfiguredEmail)
		measure(configured, configuredEmail)

		withoutPassword := measure(unconfigured, unconfiguredEmail)
		withWrongPassword := measure(configured, configuredEmail)

		// Same 4x band as the wrong-password timing test above, and for the same
		// reason: the gap being closed is three orders of magnitude.
		require.Less(t, withoutPassword, withWrongPassword*4,
			"an instance with no break-glass must not answer measurably faster")
		require.Less(t, withWrongPassword, withoutPassword*4,
			"nor measurably slower")
	})
}
