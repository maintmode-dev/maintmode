package token

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
)

// LockUserSessions takes the user's sessions lock for the transaction in ctx:
// a rotation takes it before touching the token's row, so no revocation of the
// user's sessions runs between the rotation's write and its commit.
func (s *Service) LockUserSessions(ctx context.Context, userID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.LockUserSessions")
	defer span.End()

	return s.tokensStore.LockUserSessions(ctx, userID)
}
