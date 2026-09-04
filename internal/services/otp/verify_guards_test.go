package otp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/otp"
)

// failingClaimStore fails exactly one operation and delegates the rest, so a
// test can drive a branch that a real database will not produce on demand.
type failingClaimStore struct {
	otp.Store

	claimErr error
}

// ClaimOTPAttempt is the one operation this store overrides; everything else
// falls through to the embedded real store.
func (s failingClaimStore) ClaimOTPAttempt(context.Context, uuid.UUID, int16) (bool, error) {
	return false, s.claimErr
}

// TestVerify_FailsClosedWhenTheClaimErrors is the most security-load-bearing
// branch in the file, and it is unreachable without a fake: a real database does
// not fail a single statement on request.
//
// The claim IS the ceiling. If a claim error let verification proceed, an
// attacker able to induce those errors -- a statement timeout, pool exhaustion,
// both reachable under load on an unauthenticated endpoint -- would get
// unlimited free guesses against a live code, with the counter never moving.
// Mutating the branch to continue instead of returning passed every other test.
func TestVerify_FailsClosedWhenTheClaimErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user, code, nonce := issueCode(ctx, t, svc)

	claimErr := errors.New("statement timeout")
	failing := newServiceWithStore(t, failingClaimStore{Store: credStore, claimErr: claimErr})

	_, reason, err := failing.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: code, SessionNonce: nonce,
	})

	require.ErrorIs(t, err, claimErr, "a failed claim must fail the request, not fall through")
	require.Empty(t, reason,
		"an infrastructure failure is not a judged credential and must not be audited as one")

	// And the code is still live: failing closed must not consume it.
	cred, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Nil(t, cred.ConsumedAt)
}

// TestVerify_TreatsAMissingExpiryAsDead pins a nil-guard whose flip survives
// every other test.
//
// expires_at is nullable -- the column is shared with password rows, which have
// no expiry -- and nothing in the schema ties non-null to kind='otp'. So a code
// row with a NULL expiry is representable, and reading it as "valid forever"
// would mean a code that never stops working. Dead is the only safe reading.
func TestVerify_TreatsAMissingExpiryAsDead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user := makeUser(ctx, t)

	nonce := uuid.NewString()
	cred, err := credStore.Create(ctx, &entity.AuthCredential{
		UserID:       user.ID,
		Kind:         entity.AuthCredentialKindOTP,
		SecretHash:   uuid.NewString(),
		SessionNonce: &nonce,
	})
	require.NoError(t, err)
	require.Nil(t, cred.ExpiresAt)

	_, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: "123456", SessionNonce: nonce,
	})
	require.ErrorIs(t, err, apperr.ErrInvalidCredentials)
	require.Equal(t, entity.AuditFailureCodeExpired, reason,
		"a code with no expiry must be treated as dead, never as eternal")
}

// TestVerify_RejectsACodeWithNoStoredNonce pins the other nil-guard, which has
// the sharper failure mode: session_nonce is nullable for the same reason, and
// reading a nil as "matches" would accept ANY nonce a caller sent -- defeating
// the binding the nonce exists to provide.
func TestVerify_RejectsACodeWithNoStoredNonce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := newVerifyService(t)
	user := makeUser(ctx, t)

	expiresAt := time.Now().UTC().Add(time.Minute)
	cred, err := credStore.Create(ctx, &entity.AuthCredential{
		UserID:     user.ID,
		Kind:       entity.AuthCredentialKindOTP,
		SecretHash: uuid.NewString(),
		ExpiresAt:  &expiresAt,
	})
	require.NoError(t, err)
	require.Nil(t, cred.SessionNonce)

	_, reason, err := svc.svc.Verify(ctx, &entity.VerifyOTPCmd{
		Email: user.Email, Code: "123456", SessionNonce: "anything-at-all",
	})
	require.ErrorIs(t, err, apperr.ErrOTPSessionMismatch)
	require.Equal(t, entity.AuditFailureSessionMismatch, reason,
		"a row with no stored nonce must match nothing, not everything")
}
