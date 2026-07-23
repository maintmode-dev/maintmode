package users

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// LockAdminMutations serializes mutations that can shrink the active-admin
// count by taking the shared advisory xact lock. It must be called inside the
// mutation's transaction — the lock is transaction-scoped and released on
// commit/rollback — and always after the target row's FOR UPDATE lock, so
// every caller follows the same "row lock -> advisory lock" order.
func (s *Store) LockAdminMutations(ctx context.Context) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Users.LockAdminMutations")
	defer span.End()

	return dbtx.AdvisoryXactLock(ctx, dbtx.AdvisoryLockKeyAdminMutations)
}
