package dbtx

import (
	"context"
	"fmt"
)

// AdvisoryLockKey enumerates every pg_advisory_xact_lock key used against the
// database. The database is shared by multiple modules (maintmode + auth), so
// all keys live in this single registry: independent features can never
// collide on the same lock id by accident.
//
// Lock-ordering invariant: when a single transaction acquires more than one of
// these keys, it MUST acquire them in ascending key order (admin=1 before
// seat=2). No current path co-holds both — the admin (key 1) and seat (key 2)
// guards fire on mutually exclusive transitions — but a future mutation could,
// and a fixed global order is what stops a path that takes key 2 then key 1 from
// deadlocking against one that takes key 1 then key 2. Keep new keys and their
// acquisition sites consistent with this ordering.
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

	// AdvisoryLockKeySeatMutations serializes mutations that can grant a
	// seat-occupying role (admin/reviewer/editor) so the seats-cap count-then-
	// write decision is race-free: without it two concurrent grants for the last
	// seat both count occupied<cap and both pass (write-skew), oversubscribing
	// the license. The lock is instance-wide (single-org modular monolith), taken
	// before the fresh occupancy COUNT and released on commit/rollback. It is key
	// 2 by the ordering invariant above: any tx that also takes key 1 takes it
	// first.
	AdvisoryLockKeySeatMutations AdvisoryLockKey = 2

	// AdvisoryLockKeyDEKRotation serializes DEK rotation, which scans the whole
	// data_keys table with FOR UPDATE and then re-wraps the rows it locked.
	// Concurrent rotations deadlock without it: a table-wide FOR UPDATE acquires
	// tuple locks in scan order, but a waiter that is released has to re-read the
	// updated row version and re-queue, so arrival order at a contended tuple is
	// not preserved and two rotations can end up each holding a row the other
	// needs. Every replica runs rotation at startup, so a rolling deploy has
	// several doing this at once — that is how the deadlocks show up in practice
	// (Postgres kills one rotation, and it fails startup). Taking this lock first
	// makes rotation single-writer, so the FOR UPDATE scan is uncontended.
	//
	// It is key 3 by the ordering invariant above; rotation co-holds no other key.
	AdvisoryLockKeyDEKRotation AdvisoryLockKey = 3
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
