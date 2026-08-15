package datakey

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// LockRotation serializes DEK rotation by taking the shared advisory xact lock.
// It must be called inside the rotation's transaction — the lock is
// transaction-scoped and released on commit/rollback — and before
// ListForUpdate, so a rotation never contends with another one on the tuple
// locks that scan takes.
func (s *Store) LockRotation(ctx context.Context) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.DataKeys.LockRotation")
	defer span.End()

	return dbtx.AdvisoryXactLock(ctx, dbtx.AdvisoryLockKeyDEKRotation)
}
