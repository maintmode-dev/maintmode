package authcredentials

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConsumeOTP(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	otp := makeOTP(ctx, t, user.ID)

	claimed, err := store.ConsumeOTP(ctx, otp.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	// A second consumption of the same code is not an error -- it simply did
	// not claim anything.
	claimed, err = store.ConsumeOTP(ctx, otp.ID)
	require.NoError(t, err)
	require.False(t, claimed)
}

// TestConsumeOTPConcurrent is the reason the guard lives in the WHERE clause.
// Both goroutines are released from one barrier so they genuinely contend;
// launched sequentially this test degenerates into the double-consume case and
// would pass against an implementation with no guard at all.
func TestConsumeOTPConcurrent(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	otp := makeOTP(ctx, t, user.ID)

	start := make(chan struct{})
	results := make([]bool, 2)
	errs := make([]error, 2)

	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = store.ConsumeOTP(ctx, otp.ID)
		}()
	}

	close(start)
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.NotEqual(t, results[0], results[1], "exactly one caller must claim the code")
}

// TestConsumeOTPStampsConsumption checks that consumption writes both
// timestamps and that they agree.
//
// It deliberately does not assert that updated_at moved forward of its previous
// value. Insert takes created_at and updated_at from the schema default, i.e.
// the Postgres clock, while this update writes the Go clock; the two are
// different machines whenever the database runs in a container, and the
// container clock has been observed running ahead of the host by tens of
// milliseconds. An ordering assertion across those two sources tests clock
// alignment rather than store behaviour. What the store actually promises is
// that consuming stamps the row, and that both columns get the same instant.
func TestConsumeOTPStampsConsumption(t *testing.T) {
	ctx := context.Background()
	user := makeUser(ctx, t)
	otp := makeOTP(ctx, t, user.ID)

	require.Nil(t, otp.ConsumedAt)

	claimed, err := store.ConsumeOTP(ctx, otp.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	// Read back by id: a consumed code is by definition invisible to
	// GetActiveOTPByUserID, and the store deliberately exposes no
	// get-regardless-of-state method for callers that do not need one.
	var (
		updatedAt  time.Time
		consumedAt *time.Time
	)
	err = db.QueryRowContext(ctx,
		"SELECT updated_at, consumed_at FROM auth_credentials WHERE id = $1", otp.ID,
	).Scan(&updatedAt, &consumedAt)
	require.NoError(t, err)

	require.NotNil(t, consumedAt)
	require.Equal(t, consumedAt.UTC(), updatedAt.UTC())
}
