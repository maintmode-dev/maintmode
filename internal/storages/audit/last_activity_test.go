package audit

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

var errRollbackLastActivity = errors.New("rollback last-activity test tx")

// Runs inside a rolled-back REPEATABLE READ transaction on the shared dev DB:
// a later auth event must NOT advance last activity — only whitelisted domain
// actions count.
func TestLastActivityAt_AuthActionsExcluded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	err := dbtx.NewTxManager(db).WithinTx(ctx, func(ctx context.Context) error {
		add := func(action entity.AuditAction, createdAt time.Time) {
			require.NoError(t, store.AddLog(ctx, &entity.AuditEntry{
				EventID:    xuuid.New(),
				Action:     action,
				Actor:      uniqueActor(),
				EntityID:   "e1",
				EntityType: entity.AuditEntityTypeMaintenance,
				Details:    "last-activity test",
				CreatedAt:  createdAt,
			}))
		}

		domainAt := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
		authAt := domainAt.Add(time.Hour) // newer, but must not count

		add(entity.AuditActionMaintCreated, domainAt)
		add(entity.AuditActionLoginSuccess, authAt)

		got, err := store.LastActivityAt(ctx)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.True(t, got.Equal(domainAt),
			"last activity must be the domain action (%v), not the newer auth action (%v): got %v", domainAt, authAt, got)

		return errRollbackLastActivity
	}, dbtx.WithIsolation(sql.LevelRepeatableRead))
	require.ErrorIs(t, err, errRollbackLastActivity)
}
