package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestAddMentions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	store := NewStore(db)

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))
		want := []uuid.UUID{xuuid.New(), xuuid.New()}

		require.NoError(t, store.AddMentions(ctx, maint.ID, want))

		got, err := store.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("duplicate pair does not fail or duplicate", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))
		userID := xuuid.New()

		require.NoError(t, store.AddMentions(ctx, maint.ID, []uuid.UUID{userID}))
		require.NoError(t, store.AddMentions(ctx, maint.ID, []uuid.UUID{userID}))

		got, err := store.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{userID}, got)
	})

	t.Run("empty input is a no-op", func(t *testing.T) {
		t.Parallel()

		// A nil maintenance id would violate the FK if a statement were sent,
		// so a passing call proves no query was executed.
		require.NoError(t, store.AddMentions(ctx, uuid.Nil, nil))
		require.NoError(t, store.AddMentions(ctx, uuid.Nil, []uuid.UUID{}))
	})

	t.Run("cascades on maintenance delete", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))
		require.NoError(t, store.AddMentions(ctx, maint.ID, []uuid.UUID{xuuid.New()}))

		_, err := table.Maintenances.DELETE().
			WHERE(table.Maintenances.ID.EQ(postgres.UUID(maint.ID))).
			ExecContext(ctx, db)
		require.NoError(t, err)

		got, err := store.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Empty(t, got)
	})
}
