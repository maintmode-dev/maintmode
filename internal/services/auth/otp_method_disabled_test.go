package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// Criterion 3: a code issued BEFORE the method was turned off is refused after.
//
// The verify side needs its own gate for exactly this: gating only the request
// side would leave codes already in flight redeemable until they expired, so the
// switch would take effect on the code's TTL rather than on the admin's action.
func TestLoginWithOTP_RefusedWhenMethodDisabled(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, mocks := initServiceWithUnrelatedBootstrap(t)

	// The verifier would accept this code -- it is never consulted, because the
	// gate refuses first. Asserting that is the point: a code minted while the
	// method was on must stop working the moment it is turned off.
	mocks.otpVerifier.EXPECT().Verify(gomock.Any(), gomock.Any()).Times(0)

	srv.WithMethodFlags(flagsWith(map[entity.AuthMethodName]bool{
		entity.AuthMethodNameEmailOTP: false,
	}))

	_, err := srv.LoginWithOTP(ctx, &entity.VerifyOTPCmd{
		Email:        "someone@example.com",
		Code:         "123456",
		SessionNonce: xuuid.NewString(),
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
}

// Criterion 7, and the sharpest test in the set: password RESET keeps working
// while email codes are disabled.
//
// Reset runs on the same one-time-code machinery as sign-in -- the same issuer
// and the same verifier -- so a gate placed inside the otp service would catch
// both, and an instance that turned off email codes in favor of SSO would
// silently lose password recovery for everyone who still has a password.
//
// This is the ONLY test that distinguishes a correctly-placed gate from that
// one. Criteria 2 and 3 pass either way.
func TestResetPassword_SurvivesEmailOTPBeingDisabled(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, mocks := initServiceWithUnrelatedBootstrap(t)
	user := makePasswordUser(ctx, t, srv, "old-"+xuuid.NewString())

	srv.WithMethodFlags(flagsWith(map[entity.AuthMethodName]bool{
		entity.AuthMethodNameEmailOTP: false,
	}))

	// The request half still issues: reset is not sign-in.
	mocks.otpRequester.EXPECT().
		Request(gomock.Any(), user.Email).
		Return(xuuid.NewString(), nil).
		Times(1)

	_, err := srv.RequestPasswordReset(ctx, user.Email)
	require.NoError(t, err, "password recovery must not depend on email codes being offered for SIGN-IN")

	// And the confirm half still redeems.
	mocks.otpVerifier.EXPECT().
		Verify(gomock.Any(), gomock.Any()).
		Return(user, entity.AuditFailureReason(""), nil).
		Times(1)

	newPassword := "new-" + xuuid.NewString()
	err = srv.ResetPassword(ctx, &entity.ResetPasswordCmd{
		Email:        user.Email,
		Code:         "123456",
		SessionNonce: xuuid.NewString(),
		NewPassword:  newPassword,
	})
	require.NoError(t, err, "a gate inside the otp service would break this")
}
