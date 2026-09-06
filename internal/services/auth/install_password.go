package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/utils/xcripto"
)

// installPassword writes a user's password and evicts sessions -- together or
// not at all.
//
// Both ways to acquire a password land here. Splitting the two writes would
// leave a window where the password is new but the old sessions still work,
// which is precisely what someone replacing a leaked password is closing.
//
// revokeSessions is what differs between the two callers: a change spares the
// session that made it, a reset spares nothing. It runs inside the same
// transaction as the write, so a failed revocation rolls the new password back
// rather than leaving it installed with the old sessions alive.
func (s *Service) installPassword(
	ctx context.Context,
	userID uuid.UUID,
	newPassword string,
	revokeSessions func(txCtx context.Context) error,
) error {
	hash, err := xcripto.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if txErr := s.passwords.UpsertPassword(txCtx, userID, hash); txErr != nil {
			return fmt.Errorf("write password: %w", txErr)
		}

		return revokeSessions(txCtx)
	})
}
