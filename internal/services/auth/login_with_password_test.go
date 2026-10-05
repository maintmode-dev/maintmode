package auth

import (
	"context"
	"sync"
	"testing"

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

func TestLoginWithPassword(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

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

	// Break-glass has its own endpoint. Its password must not open this one,
	// not even against the break-glass account's own address: that would be a
	// second door, gated by the email_password flag instead of nothing.
	t.Run("the break-glass password does not sign in here", func(t *testing.T) {
		t.Parallel()

		breakGlass := newTestBreakGlass("the-break-glass-" + xuuid.NewString())
		srv, _ := initServiceWithBreakGlass(t, breakGlass, nil)
		_, err := srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{Password: breakGlass.password})
		require.NoError(t, err, "the account must exist for the address to mean anything")

		_, err = srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: breakGlass.email, Password: breakGlass.password, ClientIP: "10.0.0.1",
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
		t.Parallel()

		password := "her-own-password-" + xuuid.NewString()
		srv, _ := initServiceWithUnrelatedBootstrap(t)
		user := makePasswordUser(ctx, t, srv, password)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: password, ClientIP: "10.0.0.1", UserAgent: "curl/8",
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

		srv, _ := initServiceWithUnrelatedBootstrap(t)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: "nobody-" + xuuid.NewString() + "@example.com", Password: "wrong", ClientIP: "10.0.0.2",
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
func TestLoginWithPassword_AuditNamesTheMethod(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	// The mutation guard for the case above: a constant wired in one place
	// satisfies the bootstrap assertion and fails here.
	t.Run("a stored-password success is labeled password", func(t *testing.T) {
		t.Parallel()

		password := "her-own-password-" + xuuid.NewString()
		srv, _ := initServiceWithUnrelatedBootstrap(t)
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

	// The other half of the address gate: a wrong address is equally unlabeled,
	// so the two cannot be told apart by method either.
	t.Run("a wrong address is not labeled", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)
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

	// Issuance refused after a correct stored password: the credential
	// verified, so the failure is labeled.
	t.Run("issuance refused after a correct stored password is labeled password", func(t *testing.T) {
		t.Parallel()

		password := "her-own-password-" + xuuid.NewString()
		srv, _ := initServiceWithUnrelatedBootstrap(t)
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
		// Blocked, not "this deployment cannot mint tokens" -- see
		// issuanceFailureReason. The method is the subject here; the reason is
		// asserted so this does not silently go back to the generic one.
		require.Equal(t, entity.AuditFailureUserBlocked, failed.Meta.FailureReason)
		require.Equal(t, entity.AuditLoginMethodPassword, failed.Meta.LoginMethod)
	})

	// A user whose own password is wrong IS labeled: the rule is "a credential
	// verified or was tried against a credential this user actually has", not
	// "the address matched". Nothing is disclosed -- the address is the user's
	// own, and the trail already carries it as the actor.
	t.Run("a wrong stored password is labeled password", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)
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
