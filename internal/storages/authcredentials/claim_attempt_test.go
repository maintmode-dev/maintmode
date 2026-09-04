package authcredentials

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testMaxAttempts = 5

func TestClaimOTPAttempt(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	otp := makeOTP(ctx, t, user.ID)

	claimed, err := store.ClaimOTPAttempt(ctx, otp.ID, testMaxAttempts)
	require.NoError(t, err)
	require.True(t, claimed)

	// The count is read back from the row rather than returned: a caller only
	// needs to know whether a guess was bought, and every audit reason follows
	// from that boolean.
	got, err := store.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, got.Attempts, "a claim must increment the row")
}

// TestClaimOTPAttemptStopsAtCeiling walks a row to the ceiling and past it.
//
// The boundary is the thing under test: with max = 5 the claimable values are
// 0..4, the row settles at 5, and the sixth call claims nothing. An off-by-one
// either hands out a sixth guess or bars the fifth.
func TestClaimOTPAttemptStopsAtCeiling(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	otp := makeOTP(ctx, t, user.ID)

	for i := 1; i <= testMaxAttempts; i++ {
		claimed, err := store.ClaimOTPAttempt(ctx, otp.ID, testMaxAttempts)
		require.NoError(t, err)
		require.True(t, claimed, "claim %d must succeed", i)
	}

	claimed, err := store.ClaimOTPAttempt(ctx, otp.ID, testMaxAttempts)
	require.NoError(t, err)
	require.False(t, claimed, "the ceiling must refuse the next claim")
	got, err := store.GetUnconsumedOTPByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.EqualValues(t, testMaxAttempts, got.Attempts,
		"a refused claim must not push the counter past the ceiling")
}

// TestClaimOTPAttemptConcurrent is the reason the ceiling lives in the WHERE
// clause rather than in a read-then-write.
//
// All goroutines are released from one barrier so they genuinely contend. A
// separate check-then-increment lets every one of them pass a stale
// `attempts < max` gate and go on to compare a guess -- the counter still lands
// at max, so a sequential test sees nothing wrong. What must hold is that the
// number of SUCCESSFUL claims never exceeds max, because each successful claim
// is what buys a comparison against a live code.
func TestClaimOTPAttemptConcurrent(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	otp := makeOTP(ctx, t, user.ID)

	const racers = 12

	start := make(chan struct{})
	results := make([]bool, racers)
	errs := make([]error, racers)

	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = store.ClaimOTPAttempt(ctx, otp.ID, testMaxAttempts)
		}()
	}

	close(start)
	wg.Wait()

	granted := 0
	for i := range results {
		require.NoError(t, errs[i])
		if results[i] {
			granted++
		}
	}

	require.Equal(t, testMaxAttempts, granted,
		"exactly max claims may succeed no matter how many callers contend")
}

// TestClaimOTPAttemptLeavesPasswordAlone pins the kind conjunct, mirroring
// TestConsumeOTPLeavesPasswordAlone. A password row also has consumed_at NULL
// and attempts 0, so without the conjunct this statement increments it and
// reports success -- silently, since the password getter filters on neither.
func TestClaimOTPAttemptLeavesPasswordAlone(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	password := makePassword(ctx, t, user.ID)

	claimed, err := store.ClaimOTPAttempt(ctx, password.ID, testMaxAttempts)
	require.NoError(t, err)
	require.False(t, claimed)

	got, err := store.GetPasswordByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.EqualValues(t, 0, got.Attempts, "a password must not accumulate otp attempts")
}

// TestClaimOTPAttemptIgnoresConsumed pins the consumed_at conjunct: a redeemed
// code is finished, and letting attempts accrue on it would let a caller keep
// writing to a row whose secret no longer matters.
func TestClaimOTPAttemptIgnoresConsumed(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	otp := makeOTP(ctx, t, user.ID)

	claimed, err := store.ConsumeOTP(ctx, otp.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	claimed, err = store.ClaimOTPAttempt(ctx, otp.ID, testMaxAttempts)
	require.NoError(t, err)
	require.False(t, claimed)
}

// TestClaimOTPAttemptStampsUpdatedAt pins the second column the claim writes.
//
// Nothing else reads it, so dropping UpdatedAt from the UPDATE's column list
// would break no other test here -- and the pairing of columns to values in a
// Jet row-assignment is exactly the kind of thing a later edit reorders. The
// sibling ConsumeOTP test exists for the same reason.
//
// It deliberately does not assert the value moved FORWARD of the row's previous
// updated_at: the insert takes that from the Postgres clock while this update
// writes the Go clock, and comparing across those two sources tests clock
// alignment rather than store behavior. What is asserted is that the claim
// stamped a fresh value at all.
func TestClaimOTPAttemptStampsUpdatedAt(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	otp := makeOTP(ctx, t, user.ID)

	var before time.Time
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT updated_at FROM auth_credentials WHERE id = $1", otp.ID).Scan(&before))

	claimed, err := store.ClaimOTPAttempt(ctx, otp.ID, testMaxAttempts)
	require.NoError(t, err)
	require.True(t, claimed)

	var after time.Time
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT updated_at FROM auth_credentials WHERE id = $1", otp.ID).Scan(&after))

	require.NotEqual(t, before, after, "a claim must stamp updated_at")
}
