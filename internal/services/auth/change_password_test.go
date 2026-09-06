package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xcripto"
)

func TestChangePassword(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("sets a first password when none exists", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)
		user := makePasswordUser(ctx, t, nil, "")

		require.NoError(t, srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID:      user.ID,
			NewPassword: "my-brand-new-password",
			ClientIP:    "10.0.0.1",
		}))

		_, err := srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
			Email: user.Email, Password: "my-brand-new-password", ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)
	})

	// A stolen access token must not be enough to rewrite the credential it was
	// minted from.
	t.Run("replacing a password requires proving the old one", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)
		user := makePasswordUser(ctx, t, srv, "the-original-password")

		err := srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID: user.ID, NewPassword: "a-replacement-password", ClientIP: "10.0.0.1",
		})
		require.ErrorIs(t, err, apperr.ErrValidation, "the current password is required")

		err = srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID:          user.ID,
			CurrentPassword: "not-the-original",
			NewPassword:     "a-replacement-password",
			ClientIP:        "10.0.0.1",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials)

		require.NoError(t, srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID:          user.ID,
			CurrentPassword: "the-original-password",
			NewPassword:     "a-replacement-password",
			ClientIP:        "10.0.0.1",
		}))
	})

	// Supplying one when none is set tells the client it misread the state,
	// rather than silently ignoring the field.
	t.Run("supplying a current password when none exists is refused", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)
		user := makePasswordUser(ctx, t, nil, "")

		err := srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID:          user.ID,
			CurrentPassword: "there-is-no-current-one",
			NewPassword:     "my-brand-new-password",
			ClientIP:        "10.0.0.1",
		})
		require.ErrorIs(t, err, apperr.ErrValidation)
	})

	t.Run("the length policy is enforced", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)
		user := makePasswordUser(ctx, t, nil, "")

		err := srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID: user.ID, NewPassword: strings.Repeat("a", 11), ClientIP: "10.0.0.1",
		})
		require.ErrorIs(t, err, xcripto.ErrPasswordPolicy)
	})

	// The point of changing a password after it leaked: the other sessions go.
	t.Run("other sessions are revoked and the named one survives", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)
		user := makePasswordUser(ctx, t, srv, "the-original-password")

		keep, err := srv.IssueTokenPair(ctx, user, "10.0.0.1")
		require.NoError(t, err)
		evicted, err := srv.IssueTokenPair(ctx, user, "10.0.0.1")
		require.NoError(t, err)

		require.NoError(t, srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID:          user.ID,
			CurrentPassword: "the-original-password",
			NewPassword:     "a-replacement-password",
			RefreshToken:    keep.RefreshToken,
			ClientIP:        "10.0.0.1",
		}))

		_, err = srv.Refresh(ctx, keep.RefreshToken, "10.0.0.1")
		require.NoError(t, err, "the session that changed the password must survive")

		_, err = srv.Refresh(ctx, evicted.RefreshToken, "10.0.0.1")
		require.Error(t, err, "every other session must be evicted")
	})

	// Omitting the refresh token is what lets an admin who lost it still set a
	// password -- at the cost of their own session.
	t.Run("omitting the refresh token revokes every session", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)
		user := makePasswordUser(ctx, t, srv, "the-original-password")

		pair, err := srv.IssueTokenPair(ctx, user, "10.0.0.1")
		require.NoError(t, err)

		require.NoError(t, srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID:          user.ID,
			CurrentPassword: "the-original-password",
			NewPassword:     "a-replacement-password",
			ClientIP:        "10.0.0.1",
		}))

		_, err = srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
		require.Error(t, err, "with no session named, all of them go")
	})

	// Sparing a session that is not the caller's would let a stolen token steer
	// whose access survives a password change.
	t.Run("a refresh token belonging to someone else is refused", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithUnrelatedBootstrap(t)
		mine := makePasswordUser(ctx, t, srv, "the-original-password")
		theirs := makePasswordUser(ctx, t, srv, "their-password")

		theirPair, err := srv.IssueTokenPair(ctx, theirs, "10.0.0.1")
		require.NoError(t, err)

		err = srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID:          mine.ID,
			CurrentPassword: "the-original-password",
			NewPassword:     "a-replacement-password",
			RefreshToken:    theirPair.RefreshToken,
			ClientIP:        "10.0.0.1",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidRefreshToken)
	})
}

// The "all three writes or none" property, tested in the direction the happy
// path cannot reach: a revocation that fails must roll the password back.
//
// Without this only the success direction was covered, and the existing
// wrong-refresh-token case returns before installPassword is ever entered, so
// it proves nothing about the transaction.
func TestChangePasswordRollsBackOnRevocationFailure(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, _ := initServiceWithUnrelatedBootstrap(t)
	user := makePasswordUser(ctx, t, srv, "the-original-password")

	// A refresh token belonging to nobody: FamilyByRefreshToken fails inside
	// the transaction, after the password write and the seed retirement.
	err := srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
		UserID:          user.ID,
		CurrentPassword: "the-original-password",
		NewPassword:     "a-replacement-password",
		RefreshToken:    "not-a-refresh-token-anyone-holds",
		ClientIP:        "10.0.0.1",
	})
	require.Error(t, err)

	// The old password must still work: the write was rolled back with the
	// revocation that failed.
	_, err = srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
		Email: user.Email, Password: "the-original-password", ClientIP: "10.0.0.1",
	})
	require.NoError(t, err, "a failed revocation must roll the new password back")

	_, err = srv.LoginWithPassword(ctx, &entity.LoginWithPasswordCmd{
		Email: user.Email, Password: "a-replacement-password", ClientIP: "10.0.0.1",
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials,
		"the password that failed to commit must not authenticate")
}

// A password that breaks the length policy must answer 400, not 500. The
// endpoint is authenticated and the caller owns the account, so the reason is
// safe to state -- and a client cannot render a useful message from an internal
// error.
func TestChangePasswordPolicyFailureIsAValidationError(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, _ := initServiceWithUnrelatedBootstrap(t)
	user := makePasswordUser(ctx, t, nil, "")

	err := srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
		UserID:      user.ID,
		NewPassword: strings.Repeat("a", 11),
		ClientIP:    "10.0.0.1",
	})
	require.ErrorIs(t, err, apperr.ErrValidation,
		"the mapper turns ErrValidation into 400; without it this is a 500")
	require.ErrorIs(t, err, xcripto.ErrPasswordPolicy)
}
