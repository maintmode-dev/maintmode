package otp_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
	"github.com/ruko1202/maintmode/internal/services/messaging/scheduler"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

func TestRequest_IssuesACodeForAKnownUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, sched := newService(t)
	user := makeUser(ctx, t)

	nonce, err := svc.Request(ctx, user.Email)
	require.NoError(t, err)
	require.NotEmpty(t, nonce)

	cred, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, entity.AuthCredentialKindOTP, cred.Kind)
	require.NotNil(t, cred.SessionNonce)

	// Stored raw, so the verify path can compare the returned value against the
	// column directly.
	require.Equal(t, nonce, *cred.SessionNonce)

	require.NotNil(t, cred.ExpiresAt, "a code without an expiry never stops working")
	require.True(t, cred.ExpiresAt.After(time.Now().UTC()))
	require.Zero(t, cred.Attempts)
	require.Nil(t, cred.ConsumedAt)

	payload := sched.only(t)
	require.Equal(t, cred.ID, payload.CredentialID)
	require.Equal(t, user.Email, payload.Target)
	require.Equal(t, *cred.ExpiresAt, payload.ExpiresAt)
}

// The queue must never hold a readable code: goque_task rows are never pruned,
// so a dump would otherwise keep an authentication secret indefinitely, beside a
// table that deliberately stores only its hash.
//
// Driven through the REAL scheduler and asserted against the row actually
// written to goque_task. A fake scheduler would only prove that a struct the
// test built survives a round trip through encoding/json -- the thing at risk is
// the stored JSONB, so that is what this reads.
func TestRequest_QueueHoldsNoReadableCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := goque.NewStorage(db)
	require.NoError(t, err)

	svc := newServiceWith(t, scheduler.NewService(goque.NewTaskQueueManager(storage)))
	user := makeUser(ctx, t)

	_, err = svc.Request(ctx, user.Email)
	require.NoError(t, err)

	cred, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)

	var raw []byte
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT payload FROM goque_task WHERE type = $1 AND payload->>'credential_id' = $2",
		entity.ProcessorTaskOTPEmailSend, cred.ID.String(),
	).Scan(&raw))

	var stored entity.ProcessorTaskPayloadOTPEmail
	require.NoError(t, json.Unmarshal(raw, &stored))
	require.Equal(t, cred.ID, stored.CredentialID)
	require.Equal(t, user.Email, stored.Target)

	// The code is recoverable only by holding the KEK.
	dek, err := testKeyring(t).UnwrapDEK(stored.DEK, stored.KEKURI)
	require.NoError(t, err)

	code, err := secrets.NewAESCipher().Decrypt(
		dek, stored.Code, secrets.OTPCodeAAD(stored.CredentialID.String()),
	)
	require.NoError(t, err)
	require.Len(t, string(code), 6)

	// And the stored row commits to that code without containing it: the hash
	// matches, which is a far stronger statement than "the six digits do not
	// appear in the ciphertext" -- a substring search over a few hundred bytes of
	// binary hits by chance often enough to be useless in both directions.
	require.Equal(t, xhash.HashSha256(code), cred.SecretHash)
}

// An unknown address must be indistinguishable from a known one: no row, no
// task, but still a nonce, because a nonce returned only for real accounts would
// itself answer the question the 202 exists to hide.
func TestRequest_UnknownUserLooksIdentical(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, sched := newService(t)

	nonce, err := svc.Request(ctx, uuid.NewString()+"@email.com")
	require.NoError(t, err)
	require.NotEmpty(t, nonce)
	require.Empty(t, sched.recorded())
}

// Blocked accounts take the same silent path -- telling them apart would leak
// both that the account exists and what state it is in.
func TestRequest_BlockedUserGetsNoCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, sched := newService(t)
	user := makeUser(ctx, t)

	blockedAt := time.Now().UTC()
	user.BlockedAt = &blockedAt
	require.NoError(t, usersStore.Update(ctx, user))

	nonce, err := svc.Request(ctx, user.Email)
	require.NoError(t, err)
	require.NotEmpty(t, nonce)
	require.Empty(t, sched.recorded())

	_, err = credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound)
}

// Requesting again supersedes the previous code: the partial unique index allows
// exactly one live code per user, so the old row must be consumed to free it.
func TestRequest_ReissueConsumesThePreviousCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, sched := newService(t)
	user := makeUser(ctx, t)

	_, err := svc.Request(ctx, user.Email)
	require.NoError(t, err)
	first, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)

	_, err = svc.Request(ctx, user.Email)
	require.NoError(t, err)
	second, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)

	require.NotEqual(t, first.ID, second.ID)
	require.Len(t, sched.recorded(), 2, "each issuance queues its own delivery")

	// Read the superseded row directly: it is consumed, not deleted, so the
	// retired attempt stays on record. GetUnconsumedOTPByUserID cannot show this
	// -- a consumed row is by definition invisible to it, so asserting through it
	// would only re-prove the partial unique index.
	var consumedAt *time.Time
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT consumed_at FROM auth_credentials WHERE id = $1", first.ID,
	).Scan(&consumedAt))
	require.NotNil(t, consumedAt, "reissue must consume the previous code, not drop it")

	// Two emails go out and only the newest code works, which is why the copy
	// points at the newest one.
	tasks := sched.recorded()
	require.NotEqual(t, tasks[0].idempotencyKey, tasks[1].idempotencyKey)
}

// Two simultaneous requests for one user. The active-OTP index permits one live
// row; the loser's whole transaction rolls back, including its consume, so the
// winner's code survives intact.
func TestRequest_ConcurrentReissueLeavesOneLiveCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newService(t)
	user := makeUser(ctx, t)

	var wg sync.WaitGroup
	errs := make([]error, 2)

	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = svc.Request(ctx, user.Email)
		}()
	}
	wg.Wait()

	// One request may lose the race and report the conflict; both failing would
	// mean nobody got a code, which is a real failure.
	require.False(t, errs[0] != nil && errs[1] != nil,
		"both concurrent requests failed: %v / %v", errs[0], errs[1])

	cred, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err, "exactly one live code must remain")
	require.NotNil(t, cred)
}

// A failed enqueue must take the credential with it: a code nobody will ever be
// sent should not exist.
func TestRequest_FailedEnqueueRollsBackTheCredential(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, sched := newService(t)
	user := makeUser(ctx, t)
	sched.err = errors.New("queue unavailable")

	_, err := svc.Request(ctx, user.Email)
	require.Error(t, err)

	_, err = credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound)
}

// The sealing step runs inside the transaction, and WrapDEK reaches the KMS --
// the only network call in there. Its failure must unwind the whole thing, or
// the row would outlive the delivery task and leave a code that is stored but
// can never be sent.
//
// The comment on the enqueue step promises this ("a failed wrap above, which
// calls the KMS, rolls the credential back"); only the enqueue half of that
// promise was tested before.
func TestRequest_FailedWrapRollsBackTheCredential(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	user := makeUser(ctx, t)
	svc := newServiceWithCrypto(t,
		failingKeyring{err: errors.New("kms unreachable")},
		secrets.NewAESCipher(),
	)

	_, err := svc.Request(ctx, user.Email)
	require.Error(t, err)

	_, err = credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound)
}

func TestRequest_FailedEncryptRollsBackTheCredential(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	user := makeUser(ctx, t)
	svc := newServiceWithCrypto(t, testKeyring(t), failingCipher{err: errors.New("seal failed")})

	_, err := svc.Request(ctx, user.Email)
	require.Error(t, err)

	_, err = credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.ErrorIs(t, err, apperr.ErrAuthCredentialNotFound)
}

// A failure partway through must also leave the user's PREVIOUS code intact.
// The transaction consumes it before inserting the replacement, so a rollback
// that failed to undo that consume would destroy a working code and issue
// nothing in its place -- locking the user out until the old code expired.
func TestRequest_RollbackRestoresThePreviousCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	user := makeUser(ctx, t)

	working, _ := newService(t)
	_, err := working.Request(ctx, user.Email)
	require.NoError(t, err)

	original, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)

	failing := newServiceWithCrypto(t,
		failingKeyring{err: errors.New("kms unreachable")},
		secrets.NewAESCipher(),
	)
	_, err = failing.Request(ctx, user.Email)
	require.Error(t, err)

	survivor, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err, "the previous code was consumed and not replaced")
	require.Equal(t, original.ID, survivor.ID)
	require.Nil(t, survivor.ConsumedAt)
}

// TestRequest_BurntCodeKeepsTheSlot is the test the whole attempt ceiling rests
// on, and almost all of it asserts things the caller cannot see.
//
// Without the barrier the ceiling buys nothing: burn five guesses, ask again,
// and reissue consumes the spent code and hands back a fresh one with a fresh
// counter -- "five attempts per code, unlimited codes". The response is
// deliberately identical either way, so `202 with a nonce` is true whether or
// not the guard exists. What distinguishes them is the absence of work: no new
// row, the burnt row still unconsumed, and nothing queued for delivery.
func TestRequest_BurntCodeKeepsTheSlot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, sched := newService(t)
	user := makeUser(ctx, t)

	nonce, err := svc.Request(ctx, user.Email)
	require.NoError(t, err)
	require.NotEmpty(t, nonce)

	burnt, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)

	for range svc.MaxAttempts() {
		claimed, err := credStore.ClaimOTPAttempt(ctx, burnt.ID, svc.MaxAttempts())
		require.NoError(t, err)
		require.True(t, claimed)
	}

	before := len(sched.recorded())

	// Indistinguishable from the outside: same shape, same absence of an error.
	secondNonce, err := svc.Request(ctx, user.Email)
	require.NoError(t, err)
	require.NotEmpty(t, secondNonce)

	live, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, burnt.ID, live.ID, "the burnt code must still hold the slot")
	require.Nil(t, live.ConsumedAt, "the burnt code must not have been retired")
	require.Equal(t, svc.MaxAttempts(), live.Attempts)

	require.Len(t, sched.recorded(), before, "a barred request must queue no email")
}

// TestRequest_ExpiredBurntCodeIsReplaced bounds the barrier by the code's own
// lifetime. A permanent lock on an unauthenticated endpoint would be a denial of
// service against any address an attacker knows, so once the burnt code dies the
// next request issues normally.
func TestRequest_ExpiredBurntCodeIsReplaced(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, sched := newService(t)
	user := makeUser(ctx, t)

	expired := makeExpiredOTP(ctx, t, user.ID)
	for range svc.MaxAttempts() {
		claimed, err := credStore.ClaimOTPAttempt(ctx, expired.ID, svc.MaxAttempts())
		require.NoError(t, err)
		require.True(t, claimed)
	}

	nonce, err := svc.Request(ctx, user.Email)
	require.NoError(t, err)
	require.NotEmpty(t, nonce)

	live, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.NotEqual(t, expired.ID, live.ID, "an expired burnt code must not bar reissue")
	require.Equal(t, nonce, *live.SessionNonce)
	require.Len(t, sched.recorded(), 1)
}

// TestRequest_PartiallyBurntCodeIsReplaced pins the boundary from the other
// side: below the ceiling the ordinary reissue path still applies, so a user who
// mistyped twice and asked for a new code gets one.
func TestRequest_PartiallyBurntCodeIsReplaced(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newService(t)
	user := makeUser(ctx, t)

	_, err := svc.Request(ctx, user.Email)
	require.NoError(t, err)

	first, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)

	claimed, err := credStore.ClaimOTPAttempt(ctx, first.ID, svc.MaxAttempts())
	require.NoError(t, err)
	require.True(t, claimed)

	_, err = svc.Request(ctx, user.Email)
	require.NoError(t, err)

	live, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, live.ID, "a code below the ceiling must still be replaceable")
	require.Zero(t, live.Attempts, "the replacement starts with a fresh counter")
}

// TestRequest_BlocksOnAConcurrentlyLockedCode pins the lock the barrier's read
// takes, by holding that lock from outside and observing that reissue waits.
//
// The scenario it stands in for: a verify claiming the FINAL attempt runs
// concurrently with a reissue. Unlocked, the reissue can read a count one short
// of the ceiling, conclude the code is still usable, consume it, and issue a
// replacement with a fresh counter -- turning "five guesses, then wait" into
// "five guesses, then five more" for an attacker who times the two together.
//
// Provoking that interleaving by simply racing two goroutines does not work:
// the window is a few hundred microseconds wide and the test passes with the
// lock removed, which makes it a test of nothing. So the contention is forced
// instead. A separate transaction takes the row lock and spends the final
// attempt while holding it; the reissue must then block rather than read stale
// state, and can only proceed once that transaction commits -- by which time the
// code is burnt and the slot is barred.
//
// Deleting FOR UPDATE from the store's read makes this fail: the reissue no
// longer waits, reads attempts one short of the ceiling, and retires the code.
func TestRequest_BlocksOnAConcurrentlyLockedCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newService(t)
	user := makeUser(ctx, t)

	_, err := svc.Request(ctx, user.Email)
	require.NoError(t, err)

	cred, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)

	// Spend every attempt but the last, so the claim below is the one that
	// reaches the ceiling.
	for range svc.MaxAttempts() - 1 {
		claimed, err := credStore.ClaimOTPAttempt(ctx, cred.ID, svc.MaxAttempts())
		require.NoError(t, err)
		require.True(t, claimed)
	}

	tx, err := db.BeginTxx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	// Take the row lock and spend the final attempt, holding both until commit.
	var locked int16
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT attempts FROM auth_credentials WHERE id = $1 FOR UPDATE", cred.ID).Scan(&locked))
	require.Equal(t, svc.MaxAttempts()-1, locked)

	_, err = tx.ExecContext(ctx,
		"UPDATE auth_credentials SET attempts = attempts + 1 WHERE id = $1", cred.ID)
	require.NoError(t, err)

	reissued := make(chan error, 1)
	go func() { _, e := svc.Request(ctx, user.Email); reissued <- e }()

	// The reissue must still be blocked on the lock. A read that does not wait
	// would already have retired the code by now.
	select {
	case err := <-reissued:
		require.FailNow(t, "reissue did not wait for the row lock", "returned early: %v", err)
	case <-time.After(250 * time.Millisecond):
	}

	require.NoError(t, tx.Commit())
	require.NoError(t, <-reissued)

	live, err := credStore.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, cred.ID, live.ID, "the burnt code must still hold the slot")
	require.Nil(t, live.ConsumedAt)
	require.Equal(t, svc.MaxAttempts(), live.Attempts)
}
