package otp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	mock_otp "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/otp"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// The drain loop is exercised over a mocked store: the batch sizes a real
// database would return are exactly what these tests need to script, and the
// predicate itself is covered by the store's own integration tests.
func newPruneService(t *testing.T) (*Service, *mock_otp.MockStore) {
	t.Helper()

	store := mock_otp.NewMockStore(gomock.NewController(t))

	return &Service{store: store}, store
}

func TestPrune_DrainsUntilShortBatch(t *testing.T) {
	t.Parallel()

	svc, store := newPruneService(t)

	gomock.InOrder(
		store.EXPECT().PruneOTPExpiredBefore(gomock.Any(), gomock.Any(), int64(2)).Return(int64(2), nil),
		store.EXPECT().PruneOTPExpiredBefore(gomock.Any(), gomock.Any(), int64(2)).Return(int64(2), nil),
		// A batch shorter than the limit means the table is drained for this
		// cutoff, so the loop must stop here rather than call again.
		store.EXPECT().PruneOTPExpiredBefore(gomock.Any(), gomock.Any(), int64(2)).Return(int64(1), nil),
	)

	require.NoError(t, svc.Prune(context.Background(), 24*time.Hour, 2))
}

// TestPrune_StopsAtBatchCap pins that one invocation cannot loop unbounded: with
// every batch coming back full, the loop still ends after maxPruneBatches.
func TestPrune_StopsAtBatchCap(t *testing.T) {
	t.Parallel()

	svc, store := newPruneService(t)

	store.EXPECT().
		PruneOTPExpiredBefore(gomock.Any(), gomock.Any(), int64(10)).
		Return(int64(10), nil).
		Times(maxPruneBatches)

	require.NoError(t, svc.Prune(context.Background(), 24*time.Hour, 10))
}

// TestPrune_UsesOneCutoffForEveryBatch pins that the boundary is computed once,
// so rows aging mid-sweep wait for the next tick instead of shifting the cutoff
// under the loop.
func TestPrune_UsesOneCutoffForEveryBatch(t *testing.T) {
	t.Parallel()

	svc, store := newPruneService(t)

	var seen []time.Time
	store.EXPECT().
		PruneOTPExpiredBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cutoff time.Time, limit int64) (int64, error) {
			seen = append(seen, cutoff)
			if len(seen) < 3 {
				return limit, nil
			}
			return 0, nil
		}).
		Times(3)

	require.NoError(t, svc.Prune(context.Background(), 24*time.Hour, 5))
	require.Len(t, seen, 3)
	require.Equal(t, seen[0], seen[1])
	require.Equal(t, seen[0], seen[2])
}

// TestPrune_CutoffIsAlwaysInThePast is the safety property: whatever retention
// reaches Prune -- including the non-positive values the fallback absorbs -- the
// store must never be handed a cutoff at or after now. A future cutoff would make
// a live, attempt-exhausted code eligible and reset the guess ceiling that
// claimSlot enforces by the row's continued existence.
func TestPrune_CutoffIsAlwaysInThePast(t *testing.T) {
	t.Parallel()

	for _, retention := range []time.Duration{
		-365 * 24 * time.Hour,
		-time.Second,
		0,
		time.Minute,
		24 * time.Hour,
	} {
		t.Run(retention.String(), func(t *testing.T) {
			t.Parallel()

			svc, store := newPruneService(t)

			var got time.Time
			store.EXPECT().
				PruneOTPExpiredBefore(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, cutoff time.Time, _ int64) (int64, error) {
					got = cutoff
					return 0, nil
				})

			require.NoError(t, svc.Prune(context.Background(), retention, 10))
			require.True(t, got.Before(xtime.UTCNow()),
				"cutoff %s must be strictly in the past", got)
		})
	}
}

// TestPrune_NonPositiveTunablesFallBack pins both fallbacks. The retention one
// matters most: a non-positive value must become the 24h default rather than a
// cutoff at or past now.
func TestPrune_NonPositiveTunablesFallBack(t *testing.T) {
	t.Parallel()

	svc, store := newPruneService(t)

	var gotCutoff time.Time
	var gotLimit int64
	store.EXPECT().
		PruneOTPExpiredBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cutoff time.Time, limit int64) (int64, error) {
			gotCutoff, gotLimit = cutoff, limit
			return 0, nil
		})

	before := xtime.UTCNow()
	require.NoError(t, svc.Prune(context.Background(), 0, 0))

	require.Equal(t, int64(defaultPruneBatchLimit), gotLimit)
	// The cutoff must sit a default-retention behind now, give or take the
	// microseconds the call itself takes.
	require.WithinDuration(t, before.Add(-defaultPruneRetention), gotCutoff, time.Minute)
}

func TestPrune_PropagatesStoreError(t *testing.T) {
	t.Parallel()

	svc, store := newPruneService(t)

	wantErr := errors.New("delete failed")
	store.EXPECT().
		PruneOTPExpiredBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(int64(0), wantErr)

	require.ErrorIs(t, svc.Prune(context.Background(), 24*time.Hour, 10), wantErr)
}

// observedPruneCtx returns a context whose logger writes into the returned sink,
// mirroring the seam bootstrapauth's tests use.
func observedPruneCtx(t *testing.T) (context.Context, *observer.ObservedLogs) {
	t.Helper()

	core, logs := observer.New(zapcore.DebugLevel)

	return xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zap.New(core))), logs
}

// TestPrune_LogsCompletionEvenWhenNothingDeleted defends the one place this
// sweep knowingly diverges from its invitation sibling, which wraps the same
// line in `if total > 0`.
//
// The job carries no metric, so this line is the only evidence it ran. Restoring
// the sibling's condition in the name of consistency would make a cron that
// stopped firing indistinguishable from a cron with nothing to collect -- and
// with nothing asserting on it, that regression would be silent.
func TestPrune_LogsCompletionEvenWhenNothingDeleted(t *testing.T) {
	t.Parallel()

	svc, store := newPruneService(t)
	store.EXPECT().
		PruneOTPExpiredBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(int64(0), nil)

	ctx, logs := observedPruneCtx(t)
	require.NoError(t, svc.Prune(ctx, 24*time.Hour, 10))

	done := logs.FilterMessage("pruned expired one-time codes").All()
	require.Len(t, done, 1, "a zero-row sweep must still report that it ran")
	require.Equal(t, zapcore.InfoLevel, done[0].Level)

	require.Equal(t, int64(0), done[0].ContextMap()["count"],
		"the count field is what makes the line useful")
}
