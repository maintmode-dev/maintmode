package invitationpruneprocessor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	mock_invitationpruneprocessor "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/goque_processors/invitationpruneprocessor"
)

func newPruneTask(t *testing.T, retention time.Duration, batchLimit int64) *goque.Task {
	t.Helper()
	task, err := goque.NewTaskWithPayloadAndExternalID(
		entity.ProcessorTaskInvitationPrune,
		entity.ProcessorTaskPayloadInvitationPrune{Retention: retention, BatchLimit: batchLimit},
		"test-external-id",
	)
	require.NoError(t, err)
	return task
}

func TestProcessTask_DelegatesWithPayloadTunables(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	pruner := mock_invitationpruneprocessor.NewMockPruner(ctrl)

	var gotRetention time.Duration
	var gotLimit int64
	pruner.EXPECT().
		Prune(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, retention time.Duration, batchLimit int64) error {
			gotRetention, gotLimit = retention, batchLimit
			return nil
		})

	err := NewTaskProcessor(pruner).ProcessTask(context.Background(), newPruneTask(t, 365*24*time.Hour, 1000))
	require.NoError(t, err)
	require.Equal(t, 365*24*time.Hour, gotRetention)
	require.Equal(t, int64(1000), gotLimit)
}

func TestProcessTask_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	pruner := mock_invitationpruneprocessor.NewMockPruner(ctrl)

	wantErr := errors.New("prune failed")
	pruner.EXPECT().Prune(gomock.Any(), gomock.Any(), gomock.Any()).Return(wantErr)

	err := NewTaskProcessor(pruner).ProcessTask(context.Background(), newPruneTask(t, time.Hour, 10))
	require.ErrorIs(t, err, wantErr)
}

// TestNewTaskFactory_StampsTunablesAndDayBucket asserts the factory produces a
// well-typed task whose payload survives the JSON round-trip and whose external
// id is day-bucketed.
func TestNewTaskFactory_StampsTunablesAndDayBucket(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(730*24*time.Hour, 500)(context.Background())
	require.NoError(t, err)
	require.NotNil(t, task)
	require.Equal(t, entity.ProcessorTaskInvitationPrune, task.Type)
	require.NotEmpty(t, task.ExternalID)

	ctrl := gomock.NewController(t)
	pruner := mock_invitationpruneprocessor.NewMockPruner(ctrl)

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

// TestNewTaskFactory_DefaultsZeroTunables checks the cmp.Or fallbacks.
func TestNewTaskFactory_DefaultsZeroTunables(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(0, 0)(context.Background())
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	pruner := mock_invitationpruneprocessor.NewMockPruner(ctrl)

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

func TestInvitationPruneExternalID_DayBucketed(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC)
	require.Equal(t, "invitation-prune-2026-07-22", invitationPruneExternalID(now))
}
