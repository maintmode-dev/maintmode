package dbtx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAdvisoryXactLock_OutsideTxFails asserts the loud-invariant behavior: an
// advisory xact lock taken outside a transaction dies at the end of its own
// statement, so the helper must refuse instead of silently protecting nothing.
func TestAdvisoryXactLock_OutsideTxFails(t *testing.T) {
	t.Parallel()

	err := AdvisoryXactLock(context.Background(), AdvisoryLockKeyAdminMutations)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires an active transaction")
}

// TestAdvisoryXactLock_InsideTxSucceeds asserts the lock acquires cleanly
// inside a transaction opened by the TxManager; commit releases it implicitly.
func TestAdvisoryXactLock_InsideTxSucceeds(t *testing.T) {
	t.Parallel()

	mngr := NewTxManager(db)
	err := mngr.WithinTx(context.Background(), func(ctx context.Context) error {
		return AdvisoryXactLock(ctx, AdvisoryLockKeyAdminMutations)
	})
	require.NoError(t, err)
}

// TestAdvisoryXactLock_NestedTxKeepsLockUntilOuterCommit proves a lock taken
// from a nested WithinTx attaches to the OUTER transaction. The guard that
// relies on this lock can be reached through service composition (an outer
// transactional service calling the user service); if the nested WithinTx ever
// opened and committed its own transaction instead of joining, the xact-scoped
// lock would die at the inner commit and the guard would silently protect
// nothing for the rest of the outer transaction.
func TestAdvisoryXactLock_NestedTxKeepsLockUntilOuterCommit(t *testing.T) {
	t.Parallel()

	mngr := NewTxManager(db)
	err := mngr.WithinTx(context.Background(), func(ctx context.Context) error {
		// Take the lock from a nested (joined) WithinTx scope.
		if err := mngr.WithinTx(ctx, func(ctx context.Context) error {
			return AdvisoryXactLock(ctx, AdvisoryLockKeyAdminMutations)
		}); err != nil {
			return err
		}

		// Back in the outer transaction, after the nested scope ended: this
		// backend must still hold the advisory lock. The pg_locks probe is
		// scoped to our own backend pid, so concurrent holders of the same key
		// in other tests cannot make it pass or fail spuriously.
		tx, ok := TxFromContext(ctx)
		require.True(t, ok, "outer transaction must still be in the context")

		// pg_advisory_xact_lock(bigint) splits the key across pg_locks columns:
		// high 32 bits land in classid, low 32 bits in objid. Compute both from
		// the key so the probe stays correct for any future registry entry.
		key := int64(AdvisoryLockKeyAdminMutations)
		var held bool
		err := tx.GetContext(ctx, &held, `
			SELECT EXISTS (
				SELECT 1
				FROM pg_locks
				WHERE locktype = 'advisory'
				  AND pid = pg_backend_pid()
				  AND granted
				  AND classid = $1
				  AND objid = $2
			)`, uint32(key>>32), uint32(key)) //nolint:gosec // 32-bit halves of the registry key, no overflow semantics involved
		require.NoError(t, err)
		require.True(t, held, "advisory lock taken in a nested tx scope must survive until the outer commit")
		return nil
	})
	require.NoError(t, err)
}
