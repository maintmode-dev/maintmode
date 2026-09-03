package auth

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
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

// bootstrapClaims are what the break-glass method reports on success. The
// subject is a constant in production; these tests share a database, so each
// gets its own to stand in for a separate instance.
func bootstrapClaims() *entity.OAuthIDTokenClaims {
	id := xuuid.NewString()
	return &entity.OAuthIDTokenClaims{
		Subject: entity.BootstrapSubject + "-" + id,
		Email:   id + "@bootstrap.test",
		Name:    "Bootstrap Admin",
	}
}

func TestLoginWithPassword(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("issues an admin token pair", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initServiceForMethod(t, entity.AuthMethodBootstrap)
		claims := bootstrapClaims()
		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), "the-password").
			Return(claims, nil)

		pair, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email:    "ignored@example.com",
			Password: "the-password",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)
		require.NotEmpty(t, pair.AccessToken)
		require.NotEmpty(t, pair.RefreshToken)

		access, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.Contains(t, access.UserRoles, entity.RoleAdmin,
			"the break-glass admin must actually be an admin")
		require.Equal(t, claims.Email, access.UserEmail,
			"the identity comes from the method's claims, not the request body")
	})

	// The email in the body belongs to the later email_password method and must
	// not steer who the break-glass admin is: otherwise whoever guessed the
	// password would choose the identity, rather than whoever runs the deployment.
	t.Run("the request body email is ignored", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initServiceForMethod(t, entity.AuthMethodBootstrap)
		claims := bootstrapClaims()
		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), gomock.Any()).
			Return(claims, nil).
			Times(2)

		first, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: "someone@example.com", Password: "pw", ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)
		second, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: "someone-else@example.com", Password: "pw", ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		firstClaims, err := srv.tokenSrv.VerifyAccessToken(ctx, first.AccessToken)
		require.NoError(t, err)
		secondClaims, err := srv.tokenSrv.VerifyAccessToken(ctx, second.AccessToken)
		require.NoError(t, err)

		require.Equal(t, firstClaims.Subject, secondClaims.Subject,
			"a repeat login must resolve the same user regardless of the body email")
		require.Equal(t, claims.Email, secondClaims.UserEmail)
	})

	t.Run("a wrong password yields no token", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initServiceForMethod(t, entity.AuthMethodBootstrap)
		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), "wrong").
			Return(nil, apperr.ErrInvalidCredentials)

		pair, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Password: "wrong", ClientIP: "10.0.0.1",
		})
		require.Nil(t, pair)
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	})

	// The blocked-user guard lives inside IssueAccessToken, so routing
	// break-glass through the same funnel means blocking cuts it off too — with
	// no new check written here.
	t.Run("a blocked bootstrap admin gets no token", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initServiceForMethod(t, entity.AuthMethodBootstrap)
		claims := bootstrapClaims()
		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), gomock.Any()).
			Return(claims, nil).
			Times(2)

		created, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Password: "pw", ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		access, err := srv.tokenSrv.VerifyAccessToken(ctx, created.AccessToken)
		require.NoError(t, err)
		userID, err := uuid.Parse(access.Subject)
		require.NoError(t, err)

		require.NoError(t, srv.usersSrv.BlockUser(ctx, &entity.BlockUserCmd{
			Actor:  entity.SystemUser,
			UserID: userID,
		}))

		pair, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Password: "pw", ClientIP: "10.0.0.1",
		})
		require.Nil(t, pair)
		require.ErrorIs(t, err, apperr.ErrUserBlocked)
	})
}

// Every use of a permanently-live break-glass credential must be on record —
// that audit trail is one of the compensating controls that make an
// always-present emergency login acceptable.
func TestLoginWithPassword_Audit(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("a success is audited", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initServiceForMethod(t, entity.AuthMethodBootstrap)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), gomock.Any()).
			Return(bootstrapClaims(), nil)

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Password: "pw", ClientIP: "10.0.0.1", UserAgent: "curl/8",
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

		srv, mocks := initServiceForMethod(t, entity.AuthMethodBootstrap)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		mocks.authMethod.EXPECT().
			Authenticate(gomock.Any(), gomock.Any()).
			Return(nil, apperr.ErrInvalidCredentials)

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: "claimed@example.com", Password: "wrong", ClientIP: "10.0.0.2",
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
