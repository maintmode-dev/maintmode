package authcredentials

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestUpsertPasswordCreatesThenReplaces(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)

	require.NoError(t, store.UpsertPassword(ctx, user.ID, "$argon2id$first"))

	cred, err := store.GetPasswordByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "$argon2id$first", cred.SecretHash)

	// The second write must replace rather than conflict: Create cannot be used
	// here because the partial unique index rejects the second row.
	require.NoError(t, store.UpsertPassword(ctx, user.ID, "$argon2id$second"))

	cred, err = store.GetPasswordByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "$argon2id$second", cred.SecretHash)
}

// The row is shared with one-time codes, so the columns that belong to a code
// must be cleared when a password lands on it. Leaving a stale attempts count
// or consumed_at behind is how a live credential ends up carrying OTP state.
func TestUpsertPasswordClearsCodeShapedColumns(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)

	expiresAt := time.Now().UTC().Add(time.Hour)
	nonce := "a-session-nonce"
	_, err := store.Create(ctx, &entity.AuthCredential{
		UserID:       user.ID,
		Kind:         entity.AuthCredentialKindPassword,
		SecretHash:   "$argon2id$stale",
		ExpiresAt:    &expiresAt,
		Attempts:     4,
		SessionNonce: &nonce,
	})
	require.NoError(t, err)

	require.NoError(t, store.UpsertPassword(ctx, user.ID, "$argon2id$fresh"))

	cred, err := store.GetPasswordByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "$argon2id$fresh", cred.SecretHash)
	require.Zero(t, cred.Attempts)
	require.Nil(t, cred.ExpiresAt)
	require.Nil(t, cred.ConsumedAt)
	require.Nil(t, cred.SessionNonce)
}

// A password write must not disturb a live one-time code: they share the table
// and the user, and only `kind` separates them.
func TestUpsertPasswordLeavesALiveCodeAlone(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)

	otp := makeOTP(ctx, t, user.ID)

	require.NoError(t, store.UpsertPassword(ctx, user.ID, "$argon2id$fresh"))

	live, err := store.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, otp.ID, live.ID)
	require.Equal(t, otp.SecretHash, live.SecretHash)
}

// Two writers racing to set a first password must both succeed: the upsert has
// no conflict to surface, and the later write simply wins.
//
// This also guards a data race that `go test -race` catches and a plain run does
// not: postgres.NULL is one package-level value shared by the whole process, and
// jet's typed wrappers (TimestampzExp, StringExp) mutate it in place. Building
// the statement with them made two concurrent calls write to the same object --
// real in production, where two people changing their passwords at once take
// this path. The columns are nulled with Raw* expressions instead.
func TestUpsertPasswordConcurrent(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)

	errs := make(chan error, 2)
	for i := range 2 {
		go func() {
			errs <- store.UpsertPassword(ctx, user.ID, "$argon2id$racer")
			_ = i
		}()
	}

	require.NoError(t, <-errs)
	require.NoError(t, <-errs)

	cred, err := store.GetPasswordByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "$argon2id$racer", cred.SecretHash)
}
