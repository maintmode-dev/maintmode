package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestGetMaintMentions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	store := NewStore(db)

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		mentions, err := store.GetMaintMentions(ctx, xuuid.New())
		require.NoError(t, err)
		require.Empty(t, mentions)
	})

	t.Run("ordered by insertion", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))

		// Insert one row at a time so created_at strictly increases and the
		// expected order is the insertion order rather than an arbitrary one.
		want := []uuid.UUID{xuuid.New(), xuuid.New(), xuuid.New()}
		for _, userID := range want {
			insertMention(ctx, t, maint.ID, userID)
		}

		got, err := store.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("scoped to maintenance", func(t *testing.T) {
		t.Parallel()

		period := entity.NewPeriod(now, now.Add(time.Minute))
		maint := makeMaint(ctx, t, store, period)
		other := makeMaint(ctx, t, store, period)

		userID := xuuid.New()
		insertMention(ctx, t, maint.ID, userID)
		insertMention(ctx, t, other.ID, xuuid.New())

		got, err := store.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{userID}, got)
	})

	t.Run("cascades on maintenance delete", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))
		insertMention(ctx, t, maint.ID, xuuid.New())

		_, err := table.Maintenances.DELETE().
			WHERE(table.Maintenances.ID.EQ(postgres.UUID(maint.ID))).
			ExecContext(ctx, db)
		require.NoError(t, err)

		got, err := store.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func insertMention(ctx context.Context, t *testing.T, maintID, userID uuid.UUID) {
	t.Helper()

	stmt := table.MaintenanceMentions.
		INSERT(
			table.MaintenanceMentions.MaintenanceID,
			table.MaintenanceMentions.UserID,
		).
		MODEL(&model.MaintenanceMentions{MaintenanceID: maintID, UserID: userID})

	_, err := stmt.ExecContext(ctx, db)
	require.NoError(t, err)
}
