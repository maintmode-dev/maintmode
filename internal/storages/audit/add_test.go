package audit

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// uniqueActor returns a per-run-unique actor so concurrent tests on the shared
// DB never read each other's rows.
func uniqueActor() string {
	return "add-test-" + xuuid.NewString()
}

func TestAddLog_WritesCreatedAtFromEntry(t *testing.T) {
	ctx := context.Background()
	store := NewStore(db)

	actor := uniqueActor()
	occurred := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	require.NoError(t, store.AddLog(ctx, &entity.AuditEntry{
		EventID:    uuid.New(),
		Action:     entity.AuditActionLoginSuccess,
		Actor:      actor,
		EntityType: entity.AuditEntityTypeUser,
		CreatedAt:  occurred,
	}))

	logs, _, err := store.GetLogs(ctx, &entity.GetAuditLogsCmd{
		Filter: &entity.AuditFilter{Actor: &actor},
		Limit:  10,
	})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	// created_at must reflect the event time the entry carried, not NOW() at
	// insert.
	require.WithinDuration(t, occurred, logs[0].CreatedAt, time.Millisecond)
}

func TestAddLog_IsIdempotentOnEventID(t *testing.T) {
	ctx := context.Background()
	store := NewStore(db)

	actor := uniqueActor()
	eventID := uuid.New()
	entry := &entity.AuditEntry{
		EventID:    eventID,
		Action:     entity.AuditActionUserBlocked,
		Actor:      actor,
		EntityType: entity.AuditEntityTypeUser,
		CreatedAt:  time.Now().UTC(),
	}

	// Two writes with the same event_id — a redelivered task — must collapse to
	// one row (ON CONFLICT event_id DO NOTHING).
	require.NoError(t, store.AddLog(ctx, entry))
	require.NoError(t, store.AddLog(ctx, entry))

	_, total, err := store.GetLogs(ctx, &entity.GetAuditLogsCmd{
		Filter: &entity.AuditFilter{Actor: &actor},
		Limit:  10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total, "duplicate event_id must not create a second row")
}

func TestAddLog_NilEventIDsDoNotConflict(t *testing.T) {
	ctx := context.Background()
	store := NewStore(db)

	actor := uniqueActor()
	// Two writes with no event_id (uuid.Nil → NULL column) must both persist:
	// a plain unique index treats NULLs as distinct, so legacy/non-outbox writes
	// never collide.
	for range 2 {
		require.NoError(t, store.AddLog(ctx, &entity.AuditEntry{
			Action:     entity.AuditActionLoginSuccess,
			Actor:      actor,
			EntityType: entity.AuditEntityTypeUser,
			CreatedAt:  time.Now().UTC(),
		}))
	}

	_, total, err := store.GetLogs(ctx, &entity.GetAuditLogsCmd{
		Filter: &entity.AuditFilter{Actor: &actor},
		Limit:  10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, total, "NULL event_ids must not conflict with each other")
}

func TestAddLog_OrdersByCreatedAtDesc(t *testing.T) {
	ctx := context.Background()
	store := NewStore(db)

	actor := uniqueActor()
	t1 := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC)

	// Insert the later event FIRST — if ordering followed insert order rather
	// than created_at, this would expose it.
	require.NoError(t, store.AddLog(ctx, &entity.AuditEntry{
		EventID: uuid.New(), Action: entity.AuditActionLoginSuccess, Actor: actor,
		EntityType: entity.AuditEntityTypeUser, CreatedAt: t2, Details: "second",
	}))
	require.NoError(t, store.AddLog(ctx, &entity.AuditEntry{
		EventID: uuid.New(), Action: entity.AuditActionLoginSuccess, Actor: actor,
		EntityType: entity.AuditEntityTypeUser, CreatedAt: t1, Details: "first",
	}))

	logs, _, err := store.GetLogs(ctx, &entity.GetAuditLogsCmd{
		Filter: &entity.AuditFilter{Actor: &actor},
		Limit:  10,
	})
	require.NoError(t, err)
	require.Len(t, logs, 2)
	// DESC by created_at: the t2 ("second") event comes first regardless of
	// insert order.
	require.Equal(t, "second", logs[0].Details)
	require.Equal(t, "first", logs[1].Details)
}

func TestAddLog_TieBreaksEqualCreatedAtDeterministically(t *testing.T) {
	ctx := context.Background()
	store := NewStore(db)

	actor := uniqueActor()
	sameTime := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)

	// Three rows with an identical created_at. Without an id tie-breaker their
	// order would be index-scan-dependent and unstable across queries.
	for range 3 {
		require.NoError(t, store.AddLog(ctx, &entity.AuditEntry{
			EventID: uuid.New(), Action: entity.AuditActionLoginSuccess, Actor: actor,
			EntityType: entity.AuditEntityTypeUser, CreatedAt: sameTime,
		}))
	}

	read := func() []string {
		logs, _, err := store.GetLogs(ctx, &entity.GetAuditLogsCmd{
			Filter: &entity.AuditFilter{Actor: &actor}, Limit: 10,
		})
		require.NoError(t, err)
		ids := make([]string, len(logs))
		for i, l := range logs {
			ids[i] = l.ID.String()
		}
		return ids
	}

	first := read()
	require.Len(t, first, 3)
	// id (uuidv7) is the tie-breaker → ordering is stable across repeated reads
	// and monotonically DESC, not arbitrary.
	require.Equal(t, first, read(), "equal-created_at ordering must be stable across queries")
	require.IsDecreasing(t, first, "id DESC tie-break must order equal-created_at rows deterministically")
}
