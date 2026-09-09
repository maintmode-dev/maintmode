package oauthdance

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
)

// PutInvitationHandle parks an invitation id behind the hash of the opaque
// handle the browser carries in its cookie.
//
// Only the id is stored, never the raw invitation token. The token is a bearer
// credential whose sha256 is the sole thing authenticating an accept, and it
// lives for the invitation's TTL — days, not minutes. Keeping it out of Valkey
// means a memory dump or a slow-log entry yields an id that is worthless
// without the database, rather than a credential that is not.
func (s *Store) PutInvitationHandle(ctx context.Context, handle string, invitationID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.OAuthDance.PutInvitationHandle")
	defer span.End()

	if err := s.db.Set(ctx, invitationKey(handle), invitationID.String(), s.invitationTTL).Err(); err != nil {
		return fmt.Errorf("put invitation handle: %w", err)
	}

	return nil
}
