package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestResetPassword(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	resetCmd := func(email string) *entity.ResetPasswordCmd {
		return &entity.ResetPasswordCmd{
			Email:        email,
			Code:         "123456",
			SessionNonce: "a-nonce",
			NewPassword:  "a-brand-new-password",
			ClientIP:     "10.0.0.1",
		}
	}

	t.Run("installs the new password and revokes every session", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initServiceWithUnrelatedBootstrap(t)
		user := makePasswordUser(ctx, t, srv, "the-old-password")

		mocks.otpVerifier.EXPECT().
			Verify(gomock.Any(), gomock.Any()).
			Return(user, entity.AuditFailureReason(""), nil)

		pair, err := srv.IssueTokenPair(ctx, user, "10.0.0.1")
		require.NoError(t, err)

		require.NoError(t, srv.ResetPassword(ctx, resetCmd(user.Email)))

		_, err = srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
		require.Error(t, err, "a reset evicts every session, including the caller's")

		_, err = srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: "a-brand-new-password", ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)
	})

	// Checking the policy after redemption would let a caller holding a guessed
	// code learn it was right, and would burn an attempt on a request the user
	// is about to retry.
	t.Run("a short password is refused without redeeming the code", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initServiceWithUnrelatedBootstrap(t)
		mocks.otpVerifier.EXPECT().Verify(gomock.Any(), gomock.Any()).Times(0)

		cmd := resetCmd("someone@example.com")
		cmd.NewPassword = strings.Repeat("a", 11)

		err := srv.ResetPassword(ctx, cmd)
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials,
			"a policy failure must be indistinguishable from a bad code")
	})

	t.Run("a rejected code fails the reset", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initServiceWithUnrelatedBootstrap(t)
		user := makePasswordUser(ctx, t, srv, "the-old-password")

		mocks.otpVerifier.EXPECT().
			Verify(gomock.Any(), gomock.Any()).
			Return(user, entity.AuditFailureInvalidCode, apperr.ErrInvalidCredentials)

		require.Error(t, srv.ResetPassword(ctx, resetCmd(user.Email)))

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: "the-old-password", ClientIP: "10.0.0.1",
		})
		require.NoError(t, err, "the old password must survive a failed reset")
	})

	// Verify does not refuse a blocked user -- blocking lives in
	// IssueAccessToken, which this path never calls.
	t.Run("a blocked user cannot reset", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initServiceWithUnrelatedBootstrap(t)
		user := makePasswordUser(ctx, t, srv, "the-old-password")
		blocked := *user
		now := xtime.UTCNow()
		blocked.BlockedAt = &now

		mocks.otpVerifier.EXPECT().
			Verify(gomock.Any(), gomock.Any()).
			Return(&blocked, entity.AuditFailureReason(""), nil)

		require.ErrorIs(t, srv.ResetPassword(ctx, resetCmd(user.Email)), apperr.ErrUserBlocked)
	})
}

func TestRequestPasswordReset(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, mocks := initServiceWithUnrelatedBootstrap(t)

	mocks.otpRequester.EXPECT().
		Request(gomock.Any(), "nobody@example.com").
		Return("a-nonce", nil)

	nonce, err := srv.RequestPasswordReset(ctx, "nobody@example.com")
	require.NoError(t, err)
	require.Equal(t, "a-nonce", nonce,
		"an unknown address still gets a nonce, so the response cannot enumerate accounts")
}

// An infrastructural failure inside Verify leaves the reason empty. It must
// still reach the audit trail: an attempt was made and refused, and a record
// that omits it reads as though nothing happened.
//
// The distinction survives -- it is recorded AS unknown rather than dressed up
// as an invalid code, which would claim a credential was judged when none was.
func TestResetPasswordAuditsAnUnnamedFailure(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, mocks := initServiceWithUnrelatedBootstrap(t)
	publisher := newRecordingAuditPublisher()
	srv.auditPublisher = publisher

	mocks.otpVerifier.EXPECT().
		Verify(gomock.Any(), gomock.Any()).
		Return(nil, entity.AuditFailureReason(""), errors.New("database is unreachable"))

	err := srv.ResetPassword(ctx, &entity.ResetPasswordCmd{
		Email:        "someone@example.com",
		Code:         "123456",
		SessionNonce: "a-nonce",
		NewPassword:  "a-brand-new-password",
		ClientIP:     "10.0.0.1",
	})
	require.Error(t, err)

	var recorded []audit.LoginFailed
	for _, a := range publisher.actions() {
		if failed, ok := a.(audit.LoginFailed); ok {
			recorded = append(recorded, failed)
		}
	}

	require.Len(t, recorded, 1, "a refused reset must be audited even with no reason to give")
	require.Equal(t, entity.AuditFailureUnknown, recorded[0].Meta.FailureReason)
}
