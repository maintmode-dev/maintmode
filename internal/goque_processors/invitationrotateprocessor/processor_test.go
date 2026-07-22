package invitationrotateprocessor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	mock_invitationrotateprocessor "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/goque_processors/invitationrotateprocessor"
)

func newRotateTask(t *testing.T, batchLimit int64) *goque.Task {
	t.Helper()
	task, err := goque.NewTaskWithPayloadAndExternalID(
		entity.ProcessorTaskInvitationRotate,
		entity.ProcessorTaskPayloadInvitationRotate{BatchLimit: batchLimit},
		"test-external-id",
	)
	require.NoError(t, err)
	return task
}

func TestProcessTask_DelegatesWithPayloadBatchLimit(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	rotater := mock_invitationrotateprocessor.NewMockRotater(ctrl)

	var gotLimit int64
	rotater.EXPECT().
		Rotate(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, batchLimit int64) error {
			gotLimit = batchLimit
			return nil
		})

	err := NewTaskProcessor(rotater).ProcessTask(context.Background(), newRotateTask(t, 500))
	require.NoError(t, err)
	require.Equal(t, int64(500), gotLimit)
}

func TestProcessTask_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	rotater := mock_invitationrotateprocessor.NewMockRotater(ctrl)

	wantErr := errors.New("rotate failed")
	rotater.EXPECT().Rotate(gomock.Any(), gomock.Any()).Return(wantErr)

	err := NewTaskProcessor(rotater).ProcessTask(context.Background(), newRotateTask(t, 10))
	require.ErrorIs(t, err, wantErr)
}

// TestNewTaskFactory_StampsBatchLimitAndDayBucket asserts the factory produces a
// well-typed task whose payload survives the JSON round-trip and whose external
// id is day-bucketed for multi-replica dedup.
func TestNewTaskFactory_StampsBatchLimitAndDayBucket(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(500)(context.Background())
	require.NoError(t, err)
	require.NotNil(t, task)
	require.Equal(t, entity.ProcessorTaskInvitationRotate, task.Type)
	require.NotEmpty(t, task.ExternalID)

	ctrl := gomock.NewController(t)
	rotater := mock_invitationrotateprocessor.NewMockRotater(ctrl)

	var gotLimit int64
	rotater.EXPECT().
		Rotate(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, batchLimit int64) error {
			gotLimit = batchLimit
			return nil
		})

	require.NoError(t, NewTaskProcessor(rotater).ProcessTask(context.Background(), task))
	require.Equal(t, int64(500), gotLimit)
}

// TestNewTaskFactory_DefaultsZeroBatchLimit checks the cmp.Or fallback: a zero
// configured batch limit is replaced by the package default.
func TestNewTaskFactory_DefaultsZeroBatchLimit(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(0)(context.Background())
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	rotater := mock_invitationrotateprocessor.NewMockRotater(ctrl)

	var gotLimit int64
	rotater.EXPECT().
		Rotate(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, batchLimit int64) error {
			gotLimit = batchLimit
			return nil
		})

	require.NoError(t, NewTaskProcessor(rotater).ProcessTask(context.Background(), task))
	require.Equal(t, int64(defaultBatchLimit), gotLimit)
}

func TestInvitationRotateExternalID_DayBucketed(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC)
	require.Equal(t, "invitation-rotate-2026-07-22", invitationRotateExternalID(now))
}
