package authcredentials

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestGetUnconsumedOTPByUserID(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	otp := makeOTP(ctx, t, user.ID)

	got, err := store.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, otp.ID, got.ID)

	claimed, err := store.ConsumeOTP(ctx, otp.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	_, err = store.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound)
}

// TestGettersDoNotCrossKinds is the query-shape half of keeping two hash
// formats in one column: neither read path may surface the other kind, or a
// password could be checked as though it were a sha256 code.
func TestGettersDoNotCrossKinds(t *testing.T) {
	ctx := context.Background()

	t.Run("password getter ignores a live code", func(t *testing.T) {
		user := makeUser(ctx, t)
		makeOTP(ctx, t, user.ID)

		_, err := store.GetPasswordByUserID(ctx, user.ID)
		require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound)
	})

	t.Run("otp getter ignores a password", func(t *testing.T) {
		user := makeUser(ctx, t)
		makePassword(ctx, t, user.ID)

		_, err := store.GetUnconsumedOTPByUserID(ctx, user.ID)
		require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound)
	})

	t.Run("each returns its own kind when both exist", func(t *testing.T) {
		user := makeUser(ctx, t)
		password := makePassword(ctx, t, user.ID)
		otp := makeOTP(ctx, t, user.ID)

		gotPassword, err := store.GetPasswordByUserID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, password.ID, gotPassword.ID)
		require.Equal(t, entity.AuthCredentialKindPassword, gotPassword.Kind)

		gotOTP, err := store.GetUnconsumedOTPByUserID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, otp.ID, gotOTP.ID)
		require.Equal(t, entity.AuthCredentialKindOTP, gotOTP.Kind)
	})
}

// TestCredentialsCascadeOnUserDelete covers the ON DELETE CASCADE clause. The
// users store has no Delete and this work must not add one, so the delete is
// issued directly -- the single place these tests reach past the store.
func TestCredentialsCascadeOnUserDelete(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	makeOTP(ctx, t, user.ID)
	makePassword(ctx, t, user.ID)

	_, err := db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	require.NoError(t, err)

	var count int
	err = db.QueryRowContext(ctx,
		"SELECT count(*) FROM auth_credentials WHERE user_id = $1", user.ID,
	).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)
}

// TestGetUnconsumedOTPByUserIDForUpdate checks the locking read returns the
// same row as its unlocked twin. The lock itself is not observable from a single
// session -- what it buys is pinned by the issuance barrier's race test, where
// two transactions actually contend.
func TestGetUnconsumedOTPByUserIDForUpdate(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	otp := makeOTP(ctx, t, user.ID)

	got, err := store.GetUnconsumedOTPByUserIDForUpdate(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, otp.ID, got.ID)

	claimed, err := store.ConsumeOTP(ctx, otp.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	_, err = store.GetUnconsumedOTPByUserIDForUpdate(ctx, user.ID)
	require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound)
}
