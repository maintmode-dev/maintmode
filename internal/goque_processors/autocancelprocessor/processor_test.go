package autocancelprocessor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	mock_autocancelprocessor "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/goque_processors/autocancelprocessor"
)

func newAutoCancelTask(t *testing.T, threshold time.Duration, limit int64) *goque.Task {
	t.Helper()
	task, err := goque.NewTaskWithPayloadAndExternalID(
		entity.ProcessorTaskMaintAutoCancel,
		entity.ProcessorTaskPayloadMaintAutoCancel{Threshold: threshold, Limit: limit},
		"test-external-id",
	)
	require.NoError(t, err)
	return task
}

func TestProcessTask_DelegatesWithPayloadTunables(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	canceller := mock_autocancelprocessor.NewMockUnStartedCanceller(ctrl)

	var gotCutoff time.Time
	var gotLimit int64
	canceller.EXPECT().
		CancelUnStarted(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cutoff time.Time, limit int64) error {
			gotCutoff, gotLimit = cutoff, limit
			return nil
		})

	before := time.Now()
	err := NewTaskProcessor(canceller).ProcessTask(context.Background(), newAutoCancelTask(t, 15*time.Minute, 100))
	require.NoError(t, err)

	require.Equal(t, int64(100), gotLimit)
	// cutoff is now-threshold, so it must be ~15 min before this call.
	require.WithinDuration(t, before.Add(-15*time.Minute), gotCutoff, time.Minute)
}

func TestProcessTask_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	canceller := mock_autocancelprocessor.NewMockUnStartedCanceller(ctrl)

	wantErr := errors.New("sweep failed")
	canceller.EXPECT().CancelUnStarted(gomock.Any(), gomock.Any(), gomock.Any()).Return(wantErr)

	err := NewTaskProcessor(canceller).ProcessTask(context.Background(), newAutoCancelTask(t, time.Minute, 10))
	require.ErrorIs(t, err, wantErr)
}

func TestNewTaskFactory_StampsTunablesIntoPayload(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(15*time.Minute, 100)(context.Background())
	require.NoError(t, err)
	require.NotNil(t, task)
	require.Equal(t, entity.ProcessorTaskMaintAutoCancel, task.Type)
	require.NotEmpty(t, task.ExternalID)

	// Feed the produced task back through the processor and assert the tunables
	// survived the JSON round-trip.
	ctrl := gomock.NewController(t)
	canceller := mock_autocancelprocessor.NewMockUnStartedCanceller(ctrl)

	var gotLimit int64
	canceller.EXPECT().
		CancelUnStarted(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ time.Time, limit int64) error {
			gotLimit = limit
			return nil
		})

	require.NoError(t, NewTaskProcessor(canceller).ProcessTask(context.Background(), task))
	require.Equal(t, int64(100), gotLimit)
}

func TestAutoCancelExternalID_BucketsToMinute(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)

	// Same minute, different seconds → same id (dedupes cross-replica ticks).
	require.Equal(t,
		autoCancelExternalID(base.Add(5*time.Second)),
		autoCancelExternalID(base.Add(55*time.Second)),
	)

	// Next minute → different id (a new task is enqueued).
	require.NotEqual(t,
		autoCancelExternalID(base),
		autoCancelExternalID(base.Add(time.Minute)),
	)
}
