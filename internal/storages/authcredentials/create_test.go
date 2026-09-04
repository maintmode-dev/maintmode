package authcredentials

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestCreateSecondLiveOTPConflicts(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	makeOTP(ctx, t, user.ID)

	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	_, err := store.Create(ctx, &entity.AuthCredential{
		UserID:     user.ID,
		Kind:       entity.AuthCredentialKindOTP,
		SecretHash: uuid.NewString(),
		ExpiresAt:  &expiresAt,
	})
	require.ErrorIs(t, err, apperr.ErrAuthCredentialConflict)
}

// TestCreateOTPAfterConsume is what proves the active-otp index carries its
// consumed_at clause. Written without it the index would still reject a second
// live code, so the conflict test above would pass either way; only reissue
// after consumption tells the two apart.
func TestCreateOTPAfterConsume(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	first := makeOTP(ctx, t, user.ID)

	claimed, err := store.ConsumeOTP(ctx, first.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	second := makeOTP(ctx, t, user.ID)
	require.NotEqual(t, first.ID, second.ID)
}

func TestCreatePasswordConflictsAndCoexistsWithOTP(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	makePassword(ctx, t, user.ID)

	_, err := store.Create(ctx, &entity.AuthCredential{
		UserID:     user.ID,
		Kind:       entity.AuthCredentialKindPassword,
		SecretHash: "$argon2id$v=19$m=65536,t=3,p=4$" + uuid.NewString(),
	})
	require.ErrorIs(t, err, apperr.ErrAuthCredentialConflict)

	// A live code alongside a password is the normal state during a reset.
	makeOTP(ctx, t, user.ID)
}

// TestCreateIgnoresConsumedAt covers the column Create deliberately does not
// insert. A caller that builds an entity from one it just read carries
// consumed_at along; honoring it would create a code born dead -- outside the
// active-code index, so blocking nothing and conflicting with nothing, and
// returned by no read path.
func TestCreateIgnoresConsumedAt(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)

	past := time.Now().UTC().Add(-time.Hour)
	expiresAt := time.Now().UTC().Add(10 * time.Minute)

	created, err := store.Create(ctx, &entity.AuthCredential{
		UserID:     user.ID,
		Kind:       entity.AuthCredentialKindOTP,
		SecretHash: uuid.NewString(),
		ExpiresAt:  &expiresAt,
		ConsumedAt: &past,
	})
	require.NoError(t, err)
	require.Nil(t, created.ConsumedAt, "a credential must never be born consumed")

	// The row is live: visible to the read path, and occupying the user's slot.
	got, err := store.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)

	_, err = store.Create(ctx, &entity.AuthCredential{
		UserID:     user.ID,
		Kind:       entity.AuthCredentialKindOTP,
		SecretHash: uuid.NewString(),
		ExpiresAt:  &expiresAt,
	})
	require.ErrorIs(t, err, apperr.ErrAuthCredentialConflict)
}

// TestCreateRejectsUnknownKind pins the kind CHECK. The user must be a real one:
// with a random user_id the foreign key rejects the row first and the test would
// pass without the CHECK existing at all, which is the hole it exists to close.
func TestCreateRejectsUnknownKind(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)

	_, err := store.Create(ctx, &entity.AuthCredential{
		UserID:     user.ID,
		Kind:       entity.AuthCredentialKind("sms"),
		SecretHash: uuid.NewString(),
	})
	require.Error(t, err)
	require.NotErrorIs(t, err, apperr.ErrAuthCredentialConflict)

	// Pin the rejection to the CHECK rather than to "some error": a future
	// foreign key or NOT NULL failure would satisfy a bare require.Error and
	// leave the constraint itself unproven.
	var pqErr *pq.Error
	require.ErrorAs(t, err, &pqErr)
	require.EqualValues(t, "23514", pqErr.Code, "expected a check_violation")
	require.Equal(t, "auth_credentials_kind_check", pqErr.Constraint)
}

// TestCreateRoundTrip asserts field by field rather than comparing whole
// entities: a require.Equal on two structs prints secret_hash into the failure
// output, and that habit carries into the tasks where the hashes are real.
func TestCreateRoundTrip(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)

	expiresAt := time.Now().UTC().Add(7 * time.Minute).Truncate(time.Microsecond)
	nonce := uuid.NewString()
	hash := uuid.NewString()

	created, err := store.Create(ctx, &entity.AuthCredential{
		UserID:       user.ID,
		Kind:         entity.AuthCredentialKindOTP,
		SecretHash:   hash,
		ExpiresAt:    &expiresAt,
		Attempts:     3,
		SessionNonce: &nonce,
	})
	require.NoError(t, err)

	require.Equal(t, user.ID, created.UserID)
	require.Equal(t, entity.AuthCredentialKindOTP, created.Kind)
	require.Equal(t, hash, created.SecretHash)
	require.Equal(t, int16(3), created.Attempts)
	require.NotNil(t, created.ExpiresAt)
	require.WithinDuration(t, expiresAt, *created.ExpiresAt, time.Millisecond)
	require.NotNil(t, created.SessionNonce)
	require.Equal(t, nonce, *created.SessionNonce)
	require.Nil(t, created.ConsumedAt)

	// Database-assigned, so asserted non-zero rather than compared to input.
	require.NotEqual(t, uuid.Nil, created.ID)
	require.False(t, created.CreatedAt.IsZero())
	require.False(t, created.UpdatedAt.IsZero())
}

// TestCreateNullableFieldsStayNull covers the password shape, where the
// code-only columns are left unset.
func TestCreateNullableFieldsStayNull(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)

	created := makePassword(ctx, t, user.ID)

	require.Nil(t, created.ExpiresAt)
	require.Nil(t, created.ConsumedAt)
	require.Nil(t, created.SessionNonce)
	require.Equal(t, int16(0), created.Attempts)
}
