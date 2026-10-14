package refreshtoken

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// LockUserSessions serializes rotating and revoking a user's refresh tokens.
// It must be called inside the transaction, before it locks any refresh_tokens
// row: see dbtx.AdvisoryLockKeyUserSessions.
func (s *Store) LockUserSessions(ctx context.Context, userID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.LockUserSessions")
	defer span.End()

	return dbtx.AdvisoryXactLockOn(ctx, dbtx.AdvisoryLockKeyUserSessions, userID.String())
}
