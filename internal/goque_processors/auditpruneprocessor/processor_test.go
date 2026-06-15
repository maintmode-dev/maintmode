package auditpruneprocessor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	mock_auditpruneprocessor "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/goque_processors/auditpruneprocessor"
)

func newAuditPruneTask(t *testing.T, retention time.Duration, batchLimit int64) *goque.Task {
	t.Helper()
	task, err := goque.NewTaskWithPayloadAndExternalID(
		entity.ProcessorTaskAuditPrune,
		entity.ProcessorTaskPayloadAuditPrune{Retention: retention, BatchLimit: batchLimit},
		"test-external-id",
	)
	require.NoError(t, err)
	return task
}

func TestProcessTask_DelegatesWithPayloadTunables(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	pruner := mock_auditpruneprocessor.NewMockPruner(ctrl)

	var gotRetention time.Duration
	var gotLimit int64
	pruner.EXPECT().
		Prune(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, retention time.Duration, batchLimit int64) error {
			gotRetention, gotLimit = retention, batchLimit
			return nil
		})

	err := NewTaskProcessor(pruner).ProcessTask(context.Background(), newAuditPruneTask(t, 365*24*time.Hour, 1000))
	require.NoError(t, err)

	require.Equal(t, 365*24*time.Hour, gotRetention)
	require.Equal(t, int64(1000), gotLimit)
}

func TestProcessTask_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	pruner := mock_auditpruneprocessor.NewMockPruner(ctrl)

	wantErr := errors.New("prune failed")
	pruner.EXPECT().Prune(gomock.Any(), gomock.Any(), gomock.Any()).Return(wantErr)

	err := NewTaskProcessor(pruner).ProcessTask(context.Background(), newAuditPruneTask(t, time.Hour, 10))
	require.ErrorIs(t, err, wantErr)
}

func TestNewTaskFactory_StampsTunablesIntoPayload(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(730*24*time.Hour, 500)(context.Background())
	require.NoError(t, err)
	require.NotNil(t, task)
	require.Equal(t, entity.ProcessorTaskAuditPrune, task.Type)
	require.NotEmpty(t, task.ExternalID)

	// Feed the produced task back through the processor and assert the tunables
	// survived the JSON round-trip.
	ctrl := gomock.NewController(t)
	pruner := mock_auditpruneprocessor.NewMockPruner(ctrl)

	var gotRetention time.Duration
	var gotLimit int64
	pruner.EXPECT().
		Prune(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, retention time.Duration, batchLimit int64) error {
			gotRetention, gotLimit = retention, batchLimit
			return nil
		})

	require.NoError(t, NewTaskProcessor(pruner).ProcessTask(context.Background(), task))
	require.Equal(t, 730*24*time.Hour, gotRetention)
	require.Equal(t, int64(500), gotLimit)
}

func TestNewTaskFactory_DefaultsWhenZero(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(0, 0)(context.Background())
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	pruner := mock_auditpruneprocessor.NewMockPruner(ctrl)

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
	require.Equal(t, int64(defaultBatchLimit), gotLimit)
}

func TestAuditPruneExternalID_BucketsToDay(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)

	// Same day, different hours → same id (dedupes cross-replica ticks).
	require.Equal(t,
		auditPruneExternalID(base),
		auditPruneExternalID(base.Add(6*time.Hour)),
	)

	// Next day → different id (a new task is enqueued).
	require.NotEqual(t,
		auditPruneExternalID(base),
		auditPruneExternalID(base.AddDate(0, 0, 1)),
	)
}
