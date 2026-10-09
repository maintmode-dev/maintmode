package refreshtokenpruneprocessor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	mock_refreshtokenpruneprocessor "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/goque_processors/refreshtokenpruneprocessor"
)

// capturePrune runs task through the processor against a mock pruner and
// returns the tunables the pruner received.
func capturePrune(t *testing.T, task *goque.Task) (gotRetention time.Duration, gotLimit int64) {
	t.Helper()

	pruner := mock_refreshtokenpruneprocessor.NewMockPruner(gomock.NewController(t))

	pruner.EXPECT().
		PruneRefreshTokens(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, retention time.Duration, batchLimit int64) error {
			gotRetention, gotLimit = retention, batchLimit
			return nil
		})

	require.NoError(t, NewTaskProcessor(pruner).ProcessTask(context.Background(), task))

	return gotRetention, gotLimit
}

func TestProcessTask_PropagatesError(t *testing.T) {
	t.Parallel()

	pruner := mock_refreshtokenpruneprocessor.NewMockPruner(gomock.NewController(t))
	wantErr := errors.New("prune failed")
	pruner.EXPECT().PruneRefreshTokens(gomock.Any(), gomock.Any(), gomock.Any()).Return(wantErr)

	task, err := NewTaskFactory(time.Hour, 10)(context.Background())
	require.NoError(t, err)

	require.ErrorIs(t, NewTaskProcessor(pruner).ProcessTask(context.Background(), task), wantErr)
}

// TestNewTaskFactory_StampsTunables asserts the tunables the factory stamps
// survive the JSON round-trip and reach the pruner unchanged. The task type is
// asserted because stamping the .cron producer type instead would enqueue
// tasks no registered processor drains.
func TestNewTaskFactory_StampsTunables(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(48*time.Hour, 500)(context.Background())
	require.NoError(t, err)
	require.Equal(t, entity.ProcessorTaskRefreshTokenPrune, task.Type)

	retention, limit := capturePrune(t, task)
	require.Equal(t, 48*time.Hour, retention)
	require.Equal(t, int64(500), limit)
}

// TestNewTaskFactory_DefaultsZeroTunables is what lets the config block be
// omitted entirely: zero values become the defaults the token service and the
// shipped configs also use.
func TestNewTaskFactory_DefaultsZeroTunables(t *testing.T) {
	t.Parallel()

	task, err := NewTaskFactory(0, 0)(context.Background())
	require.NoError(t, err)

	retention, limit := capturePrune(t, task)
	require.Equal(t, 24*time.Hour, retention)
	require.Equal(t, int64(1000), limit)
}

func TestRefreshTokenPruneExternalID_DayBucketed(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 10, 9, 3, 30, 0, 0, time.UTC)
	require.Equal(t, "refresh-token-prune-2026-10-09", refreshTokenPruneExternalID(base))
	require.Equal(t, refreshTokenPruneExternalID(base), refreshTokenPruneExternalID(base.Add(20*time.Hour)))
	require.NotEqual(t, refreshTokenPruneExternalID(base), refreshTokenPruneExternalID(base.Add(24*time.Hour)))
}
