package dbtx

import (
	"context"
	"fmt"
)

// AdvisoryLockKey enumerates every pg_advisory_xact_lock key used against the
// database. The database is shared by multiple modules (maintmode + auth), so
// all keys live in this single registry: independent features can never
// collide on the same lock id by accident.
type AdvisoryLockKey int64

const (
	// AdvisoryLockKeyAdminMutations serializes mutations that can change the
	// active-admin count (revoking the admin role, blocking an admin, replacing
	// an admin's role set). The last-admin guard counts admins under READ
	// COMMITTED while FOR UPDATE covers only the target row, so two concurrent
	// mutations targeting different admins are prone to write-skew: both count
	// the same total, both pass, and together they remove the last active
	// admin. Taking this lock before the count makes the count-then-write
	// decision race-free.
	AdvisoryLockKeyAdminMutations AdvisoryLockKey = 1
)

// AdvisoryXactLock acquires a transaction-scoped advisory lock, blocking until
// it is available. The lock is released automatically on commit or rollback.
//
// It requires an active transaction in the context and fails loudly without
// one: outside a transaction pg_advisory_xact_lock is released at the end of
// its own statement, so it would silently protect nothing.
func AdvisoryXactLock(ctx context.Context, key AdvisoryLockKey) error {
	tx, ok := TxFromContext(ctx)
	if !ok {
		return fmt.Errorf("advisory xact lock %d requires an active transaction: outside one the lock dies at the end of its own statement", key)
	}

	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", int64(key)); err != nil {
		return fmt.Errorf("acquire advisory xact lock %d: %w", key, err)
	}

	return nil
}
