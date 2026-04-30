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

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := NewStore(db)

	t.Run("ErrNoRows", func(t *testing.T) {
		t.Parallel()

		dbMaint, err := store.GetMaint(ctx, xuuid.New())
		require.EqualError(t, err, apperr.ErrMaintNotFound.Error())
		require.Nil(t, dbMaint)
	})

	t.Run("GetForUpdate", func(t *testing.T) {
		t.Parallel()

		tx := dbtx.NewTxManager(db)

		now := xtime.UTCNow()
		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))

		eg := &errgroup.Group{}
		for i := range 10 {
			eg.Go(func() error {
				return tx.WithinTx(ctx, func(ctx context.Context) error {
					dbMaint, err := store.GetMaintForUpdate(ctx, maint.ID)
					require.NoError(t, err)
					require.NotNil(t, dbMaint)

					dbMaint.Description += fmt.Sprint(i)

					return store.UpdateMaint(ctx, dbMaint)
				})
			})
		}

		require.NoError(t, eg.Wait())

		dbMaint, err := store.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.NotNil(t, dbMaint)
		require.Regexp(t, fmt.Sprintf(`^%s\d{10}$`, maint.Description), dbMaint.Description)
	})
}
