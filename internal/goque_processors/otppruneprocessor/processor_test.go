package otppruneprocessor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	mock_otppruneprocessor "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/goque_processors/otppruneprocessor"
)

func newPruneTask(t *testing.T, retention time.Duration, batchLimit int64) *goque.Task {
	t.Helper()

	task, err := goque.NewTaskWithPayloadAndExternalID(
		entity.ProcessorTaskOTPPrune,
		entity.ProcessorTaskPayloadOTPPrune{Retention: retention, BatchLimit: batchLimit},
		"test-external-id",
	)
	require.NoError(t, err)

	return task
}

func TestProcessTask_DelegatesWithPayloadTunables(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	pruner := mock_otppruneprocessor.NewMockPruner(ctrl)

	var gotRetention time.Duration
	var gotLimit int64
	pruner.EXPECT().
		Prune(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, retention time.Duration, batchLimit int64) error {
			gotRetention, gotLimit = retention, batchLimit
			return nil
		})

	err := NewTaskProcessor(pruner).ProcessTask(context.Background(), newPruneTask(t, 24*time.Hour, 1000))
	require.NoError(t, err)
	require.Equal(t, 24*time.Hour, gotRetention)
	require.Equal(t, int64(1000), gotLimit)
}

func TestProcessTask_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	pruner := mock_otppruneprocessor.NewMockPruner(ctrl)

	wantErr := errors.New("prune failed")
	pruner.EXPECT().Prune(gomock.Any(), gomock.Any(), gomock.Any()).Return(wantErr)

	err := NewTaskProcessor(pruner).ProcessTask(context.Background(), newPruneTask(t, time.Hour, 10))
	require.ErrorIs(t, err, wantErr)
}

// TestNewTaskFactory_StampsTunablesAndDayBucket asserts the tunables the factory
// stamps survive the JSON round-trip and reach the pruner unchanged.
//
// The task type is asserted because the factory chooses which constant to stamp,
// and stamping the wrong one -- the .cron producer type, say -- would enqueue
// tasks that no registered processor drains, losing the work silently. That is
// verified by mutation, not assumed.
//
// Nothing else about the task struct is asserted: NotNil after a nil error, or a
// non-empty external id, is goque echoing its own arguments back. The external
// id's actual value is pinned by TestOTPPruneExternalID_DayBucketed.
func TestNewTaskFactory_StampsTunablesAndDayBucket(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(48*time.Hour, 500)(context.Background())
	require.NoError(t, err)
	require.Equal(t, entity.ProcessorTaskOTPPrune, task.Type)

	ctrl := gomock.NewController(t)
	pruner := mock_otppruneprocessor.NewMockPruner(ctrl)

	var gotRetention time.Duration
	var gotLimit int64
	pruner.EXPECT().
		Prune(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, retention time.Duration, batchLimit int64) error {
			gotRetention, gotLimit = retention, batchLimit
			return nil
		})

	require.NoError(t, NewTaskProcessor(pruner).ProcessTask(context.Background(), task))
	require.Equal(t, 48*time.Hour, gotRetention)
	require.Equal(t, int64(500), gotLimit)
}

// TestNewTaskFactory_DefaultsZeroTunables checks the cmp.Or fallbacks, and pins
// that the retention default is the 24h the service and the deployment config
// also use -- three values that must not drift apart.
func TestNewTaskFactory_DefaultsZeroTunables(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(0, 0)(context.Background())
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	pruner := mock_otppruneprocessor.NewMockPruner(ctrl)

	var gotRetention time.Duration
	var gotLimit int64
	pruner.EXPECT().
		Prune(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, retention time.Duration, batchLimit int64) error {
			gotRetention, gotLimit = retention, batchLimit
			return nil
		})

	require.NoError(t, NewTaskProcessor(pruner).ProcessTask(context.Background(), task))
	require.Equal(t, defaultRetention, gotRetention)
	require.Equal(t, 24*time.Hour, gotRetention)
	require.Equal(t, int64(defaultBatchLimit), gotLimit)
}

func TestOTPPruneExternalID_DayBucketed(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 5, 3, 15, 0, 0, time.UTC)
	require.Equal(t, "otp-prune-2026-09-05", otpPruneExternalID(base))

	// Any other moment in the same UTC day must collapse to the same id -- that
	// is what dedupes replicas ticking the same schedule.
	require.Equal(t, otpPruneExternalID(base), otpPruneExternalID(base.Add(20*time.Hour)))
	require.NotEqual(t, otpPruneExternalID(base), otpPruneExternalID(base.Add(24*time.Hour)))
}
