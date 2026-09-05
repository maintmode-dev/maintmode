package auth

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// onlyAction asserts a single audited action and returns it.
func onlyAction(t *testing.T, p *recordingAuditPublisher) audit.Action {
	t.Helper()

	got := p.actions()
	require.Len(t, got, 1)

	return got[0]
}

// TestLoginWithOTP_AuditsAFailureAgainstAnUnknownAddress is the requirement the
// ticket is most explicit about: a sweep across addresses that do not exist must
// be visible.
//
// Those attempts have no user to attribute to, so the event carries a synthetic
// actor holding the claimed address. Without that the row lands with an empty
// actor and a zero-UUID entity id, and the sweep is invisible in exactly the
// trail meant to show it.
func TestLoginWithOTP_AuditsAFailureAgainstAnUnknownAddress(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)
	publisher := newRecordingAuditPublisher()
	srv.auditPublisher = publisher

	const claimed = "nobody@example.com"

	cmd := &entity.VerifyOTPCmd{
		Email:     claimed,
		Code:      "123456",
		ClientIP:  "203.0.113.9",
		UserAgent: "curl/8.4.0",
	}

	mocks.otpVerifier.EXPECT().
		Verify(gomock.Any(), cmd).
		Return(nil, entity.AuditFailureInvalidCode, apperr.ErrInvalidCredentials)

	_, err := srv.LoginWithOTP(t.Context(), cmd)
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials)

	failed, ok := onlyAction(t, publisher).(audit.LoginFailed)
	require.True(t, ok, "a rejected code must be audited as a failed login")
	require.Equal(t, claimed, failed.User.Email, "the claimed address is the only attribution there is")
	require.Equal(t, uuid.Nil, failed.User.ID)
	require.Equal(t, entity.AuditFailureInvalidCode, failed.Meta.FailureReason)
	require.Equal(t, "203.0.113.9", failed.Meta.IP)
	require.Equal(t, "curl/8.4.0", failed.Meta.UserAgent)
}

// TestLoginWithOTP_AuditsEachReasonDistinctly pins that the four reasons survive
// to the trail. The response collapses them into one answer, so this is the only
// place an operator can tell a brute-force run from slow mail.
func TestLoginWithOTP_AuditsEachReasonDistinctly(t *testing.T) {
	t.Parallel()

	reasons := []entity.AuditFailureReason{
		entity.AuditFailureInvalidCode,
		entity.AuditFailureAttemptsExhausted,
		entity.AuditFailureSessionMismatch,
		entity.AuditFailureCodeExpired,
	}

	for _, reason := range reasons {
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()

			srv, mocks := initService(t)
			publisher := newRecordingAuditPublisher()
			srv.auditPublisher = publisher

			user := &entity.User{ID: uuid.New(), Email: "real@example.com"}
			cmd := &entity.VerifyOTPCmd{Email: user.Email, Code: "123456", ClientIP: "10.0.0.1"}

			mocks.otpVerifier.EXPECT().
				Verify(gomock.Any(), cmd).
				Return(user, reason, apperr.ErrInvalidCredentials)

			_, err := srv.LoginWithOTP(t.Context(), cmd)
			require.Error(t, err)

			failed, ok := onlyAction(t, publisher).(audit.LoginFailed)
			require.True(t, ok)
			require.Equal(t, reason, failed.Meta.FailureReason)
			require.Equal(t, user.ID, failed.User.ID)
		})
	}
}

// TestLoginWithOTP_DoesNotAuditInfrastructuralFailures pins the other side. A
// database error is not a judged credential, and recording it as a failed
// sign-in would put noise into the trail an operator reads to spot attacks.
func TestLoginWithOTP_DoesNotAuditInfrastructuralFailures(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)
	publisher := newRecordingAuditPublisher()
	srv.auditPublisher = publisher

	cmd := &entity.VerifyOTPCmd{Email: "a@example.com", Code: "123456", ClientIP: "10.0.0.1"}

	mocks.otpVerifier.EXPECT().
		Verify(gomock.Any(), cmd).
		Return(nil, entity.AuditFailureReason(""), errors.New("connection refused"))

	_, err := srv.LoginWithOTP(t.Context(), cmd)
	require.Error(t, err)

	require.Empty(t, publisher.actions(), "an infrastructure error is not a sign-in failure")
}

// TestLoginWithOTP_AuditsSuccess pins the half a failure-only trail cannot
// answer: who signed in, from where, and on which session.
func TestLoginWithOTP_AuditsSuccess(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)

	// Provisioned through the ordinary OAuth exchange: LoginWithOTP issues real
	// tokens, so it needs a user that actually exists rather than a literal.
	oauthUser := exchangeIDTokenMock(mocks, 1)
	_, err := srv.ExchangeIDToken(t.Context(), &entity.ExchangeIDTokenCmd{
		Provider: entity.AuthMethodGoogle,
		IDToken:  "id-token",
		ClientIP: "10.0.0.1",
	})
	require.NoError(t, err)

	user, err := srv.usersSrv.GetByEmail(t.Context(), oauthUser.Email)
	require.NoError(t, err)

	publisher := newRecordingAuditPublisher()
	srv.auditPublisher = publisher

	cmd := &entity.VerifyOTPCmd{
		Email:     user.Email,
		Code:      "123456",
		ClientIP:  "198.51.100.4",
		UserAgent: "Mozilla/5.0",
	}

	mocks.otpVerifier.EXPECT().
		Verify(gomock.Any(), cmd).
		Return(user, entity.AuditFailureReason(""), nil)

	pair, err := srv.LoginWithOTP(t.Context(), cmd)
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)

	success, ok := onlyAction(t, publisher).(audit.LoginSuccess)
	require.True(t, ok, "a redeemed code must be audited as a successful login")
	require.Equal(t, user.ID, success.User.ID)
	require.Equal(t, "198.51.100.4", success.Meta.IP)
	require.Equal(t, "Mozilla/5.0", success.Meta.UserAgent)
	require.Equal(t, pair.SessionID.String(), success.Meta.SessionID)
}
