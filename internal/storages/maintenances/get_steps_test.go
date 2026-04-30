package maintenances

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestGetMaintSteps(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := NewStore(db)

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		steps, err := store.GetMaintSteps(ctx, xuuid.New())
		require.NoError(t, err)
		require.Empty(t, steps)
	})

	t.Run("GetForUpdate", func(t *testing.T) {
		t.Parallel()

		tx := dbtx.NewTxManager(db)

		now := xtime.UTCNow()
		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))
		makeSteps(ctx, t, store, maint.ID)

		eg := &errgroup.Group{}
		for i := range 10 {
			eg.Go(func() error {
				return tx.WithinTx(ctx, func(ctx context.Context) error {
					dbSteps, err := store.GetMaintStepsForUpdate(ctx, maint.ID)
					require.NoError(t, err)
					require.NotEmpty(t, dbSteps)

					step := dbSteps[0]
					step.Description += fmt.Sprint(i)

					return store.UpdateStep(ctx, maint.ID, step)
				})
			})
		}

		require.NoError(t, eg.Wait())

		steps, err := store.GetMaintSteps(ctx, maint.ID)
		require.NoError(t, err)
		require.NotEmpty(t, steps)
		step := steps[0]
		require.Regexp(t, `^Description1\d{10}$`, step.Description)
	})
}
