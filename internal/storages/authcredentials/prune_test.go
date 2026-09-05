package authcredentials

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Two separate rules govern this file, and only the first is about assertions.
//
//  1. Every assertion counts rows for one freshly created user. The suite runs
//     with -count 2 against a shared database, so a table-wide count would race
//     whatever else is running.
//
//  2. Every cutoff passed to the sweep stays in the PAST, and close to it. This
//     one is easy to miss because it is not about what a test observes but about
//     what its DELETE reaches: the statement is bounded by expires_at and limit
//     alone -- never by user_id -- so a distant cutoff deletes rows belonging to
//     whatever else shares the database, including other packages running
//     concurrently under `-p 2`. Scoping the assertions does not make a sweeping
//     cutoff safe.
func credentialExists(ctx context.Context, t *testing.T, id uuid.UUID) bool {
	t.Helper()

	var n int
	require.NoError(t, db.GetContext(ctx, &n,
		"SELECT count(*) FROM auth_credentials WHERE id = $1", id))

	return n > 0
}

func countForUser(ctx context.Context, t *testing.T, userID uuid.UUID) int {
	t.Helper()

	var n int
	require.NoError(t, db.GetContext(ctx, &n,
		"SELECT count(*) FROM auth_credentials WHERE user_id = $1", userID))

	return n
}

// TestPruneOTPExpiredBefore_DeletesOnlyRowsPastCutoff covers the core age
// boundary: a code expired before the cutoff goes, one expiring after it stays.
func TestPruneOTPExpiredBefore_DeletesOnlyRowsPastCutoff(t *testing.T) {
	ctx := context.Background()
	now := xtime.UTCNow()

	// Two users, because one live OTP per user is a partial-unique invariant.
	oldUser := makeUser(ctx, t)
	freshUser := makeUser(ctx, t)

	old := makeOTPExpiringAt(ctx, t, oldUser.ID, now.Add(-48*time.Hour))
	fresh := makeOTPExpiringAt(ctx, t, freshUser.ID, now.Add(10*time.Minute))

	deleted, err := store.PruneOTPExpiredBefore(ctx, now.Add(-24*time.Hour), 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))

	require.False(t, credentialExists(ctx, t, old.ID), "expired code must be deleted")
	require.True(t, credentialExists(ctx, t, fresh.ID), "unexpired code must survive")
}

// TestPruneOTPExpiredBefore_SparesLiveCode is the security criterion, and the
// reason it stands apart from the boundary test above.
//
// The per-code attempt ceiling is enforced by the row's continued existence:
// claimSlot refuses to issue a new code while it finds a live row with the
// attempts exhausted. Delete such a row and the next request takes the
// first-request branch and hands back a fresh code with a fresh counter, turning
// "five attempts per code" into "five attempts per code, unlimited codes".
//
// The property is checked at the near edge of the permitted range, not only at
// the 24h default: a 1-minute retention is the smallest cutoff config allows,
// and if a future-dated row survives that, it survives every larger one. The
// attempts counter is set to the ceiling so the fixture is exactly the row the
// bypass would need.
func TestPruneOTPExpiredBefore_SparesLiveCode(t *testing.T) {
	ctx := context.Background()
	now := xtime.UTCNow()

	user := makeUser(ctx, t)
	live := makeOTPExpiringAt(ctx, t, user.ID, now.Add(5*time.Minute))

	// Exhaust the guess ceiling: this is the state the barrier exists to hold.
	claimed, err := store.ClaimOTPAttempt(ctx, live.ID, 1)
	require.NoError(t, err)
	require.True(t, claimed)

	for _, retention := range []time.Duration{time.Minute, 24 * time.Hour} {
		_, err := store.PruneOTPExpiredBefore(ctx, now.Add(-retention), 100)
		require.NoError(t, err)
		require.True(t, credentialExists(ctx, t, live.ID),
			"a live code must survive a sweep at retention %s", retention)
	}
}

// TestPruneOTPExpiredBefore_SparesPasswordWithNullExpiry is the NULL barrier:
// a password row has no expiry, and NULL < cutoff is never true.
//
// The cutoff is in the PAST, deliberately. A far-future cutoff would demonstrate
// the same property, but it would also make every OTP row in the database
// eligible -- and the DELETE's reach is not scoped by user_id even though this
// file's assertions are. Under `-p 2` another package's DB-backed suite can be
// running against the same table, so a sweeping cutoff would delete fixtures out
// from under it: a flaky red in services/otp's claim tests, and worse, a silent
// green in TestVerify_ExpiredCodeIsRetired, whose "row is gone" assertion cannot
// tell a retired row from one this sweep ate. A past cutoff proves the barrier
// just as well, because the password row would be caught by it if it had any
// expiry at all.
func TestPruneOTPExpiredBefore_SparesPasswordWithNullExpiry(t *testing.T) {
	ctx := context.Background()

	user := makeUser(ctx, t)
	pwd := makePassword(ctx, t, user.ID)

	_, err := store.PruneOTPExpiredBefore(ctx, xtime.UTCNow().Add(-24*time.Hour), 100)
	require.NoError(t, err)

	require.True(t, credentialExists(ctx, t, pwd.ID))
	require.Equal(t, 1, countForUser(ctx, t, user.ID))
}

// TestPruneOTPExpiredBefore_SparesPasswordWithExpiry is the kind guard, and the
// only test that can prove it exists. Unlike the NULL-expiry case above, this row
// WOULD be deleted by a predicate that dropped `kind = 'otp'`.
func TestPruneOTPExpiredBefore_SparesPasswordWithExpiry(t *testing.T) {
	ctx := context.Background()
	now := xtime.UTCNow()

	user := makeUser(ctx, t)
	pwd := makePasswordExpiringAt(ctx, t, user.ID, now.Add(-72*time.Hour))

	deleted, err := store.PruneOTPExpiredBefore(ctx, now.Add(-24*time.Hour), 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(0))

	require.True(t, credentialExists(ctx, t, pwd.ID),
		"a password row must survive even with an expiry older than the cutoff")
	require.Equal(t, 1, countForUser(ctx, t, user.ID))
}

// TestPruneOTPExpiredBefore_ConsumedRowsFollowTheSameThreshold pins that
// consumed_at plays no part in the predicate: a consumed code leaves on age
// alone, and a consumed-but-recent code stays.
func TestPruneOTPExpiredBefore_ConsumedRowsFollowTheSameThreshold(t *testing.T) {
	ctx := context.Background()
	now := xtime.UTCNow()

	agedUser := makeUser(ctx, t)
	recentUser := makeUser(ctx, t)

	aged := makeOTPExpiringAt(ctx, t, agedUser.ID, now.Add(-48*time.Hour))
	recent := makeOTPExpiringAt(ctx, t, recentUser.ID, now.Add(-time.Minute))

	// consumed_at is excluded from Create's column list by design, so the state
	// is reached through ConsumeOTP. It does not check expiry, so an already-aged
	// row can be consumed.
	for _, id := range []uuid.UUID{aged.ID, recent.ID} {
		ok, err := store.ConsumeOTP(ctx, id)
		require.NoError(t, err)
		require.True(t, ok)
	}

	_, err := store.PruneOTPExpiredBefore(ctx, now.Add(-24*time.Hour), 100)
	require.NoError(t, err)

	require.False(t, credentialExists(ctx, t, aged.ID),
		"a consumed code past the cutoff leaves through the same sweep")
	require.True(t, credentialExists(ctx, t, recent.ID),
		"being consumed grants no early deletion")
}

// TestPruneOTPExpiredBefore_RespectsLimit pins that one call never removes more
// than the batch bound, which is what makes the service's drain loop meaningful.
func TestPruneOTPExpiredBefore_RespectsLimit(t *testing.T) {
	ctx := context.Background()
	now := xtime.UTCNow()

	// One OTP per user, because one live OTP per user is a partial-unique
	// invariant, so a surviving id is exactly a surviving row.
	ids := make([]uuid.UUID, 0, 3)
	for range 3 {
		u := makeUser(ctx, t)
		ids = append(ids, makeOTPExpiringAt(ctx, t, u.ID, now.Add(-48*time.Hour)).ID)
	}

	deleted, err := store.PruneOTPExpiredBefore(ctx, now.Add(-24*time.Hour), 2)
	require.NoError(t, err)
	require.LessOrEqual(t, deleted, int64(2), "one call must not exceed the limit")

	remaining := 0
	for _, id := range ids {
		if credentialExists(ctx, t, id) {
			remaining++
		}
	}
	require.GreaterOrEqual(t, remaining, 1,
		"with 3 eligible rows and a limit of 2, at least one must be left for the next batch")

	// Drain the rest so the fixtures do not linger.
	_, err = store.PruneOTPExpiredBefore(ctx, now.Add(-24*time.Hour), 100)
	require.NoError(t, err)
	for _, id := range ids {
		require.False(t, credentialExists(ctx, t, id))
	}
}
