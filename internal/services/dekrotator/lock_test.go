package dekrotator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Rotation scans data_keys with a table-wide FOR UPDATE, and two concurrent
// scans deadlock on those tuple locks: a released waiter re-reads the updated
// row and re-queues, so arrival order at a contended tuple is not preserved.
// Every replica rotates at startup, so a rolling deploy triggers this and
// Postgres kills one rotation — failing that replica's startup. The advisory
// lock makes rotation single-writer so the scan is uncontended.
//
// This asserts the lock is actually held during rotation rather than racing two
// rotations and hoping to observe a deadlock: the second rotation would find
// every row already on the active KEK and re-wrap nothing, so it never writes
// concurrently and the race does not reproduce from Go.
func TestRotator_HoldsAdvisoryLockDuringRotation(t *testing.T) {
	ctx := context.Background()
	kek1, kek2 := uniqueKEKURI("lock-1"), uniqueKEKURI("lock-2")

	seedDEK(ctx, t, keyringWith(t, kek1, kek1), "lock-probe")
	rot := NewRotator(txManager, store, keyringWith(t, kek2, kek1, kek2))

	// Observe from inside the rotation's own transaction: the lock is
	// transaction-scoped, so it is only visible while that transaction is open.
	var held bool
	err := txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if lockErr := store.LockRotation(txCtx); lockErr != nil {
			return lockErr
		}

		tx, ok := dbtx.TxFromContext(txCtx)
		require.True(t, ok)

		return tx.QueryRowxContext(txCtx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_locks
				WHERE locktype = 'advisory'
				  AND objid = $1
				  AND granted
				  AND pid = pg_backend_pid()
			)`, int64(dbtx.AdvisoryLockKeyDEKRotation)).Scan(&held)
	})
	require.NoError(t, err)
	require.True(t, held, "rotation must hold the DEK-rotation advisory lock")

	// The lock is released with the transaction and does not leak past it.
	_, _, err = rot.Rotate(ctx)
	require.NoError(t, err)

	var stillHeld bool
	require.NoError(t, db.QueryRowx(`
		SELECT EXISTS (
			SELECT 1 FROM pg_locks
			WHERE locktype = 'advisory' AND objid = $1 AND granted
		)`, int64(dbtx.AdvisoryLockKeyDEKRotation)).Scan(&stillHeld))
	require.False(t, stillHeld, "advisory lock must be released when rotation commits")
}
