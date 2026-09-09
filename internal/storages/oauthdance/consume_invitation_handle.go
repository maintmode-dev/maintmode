package oauthdance

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	valkeylib "github.com/redis/go-redis/v9"
	"github.com/ruko1202/xlog"
)

// ConsumeInvitationHandle redeems a handle for the invitation id behind it,
// returning nil when there is nothing to redeem.
//
// A nil id is NOT an error. /start stores a handle whenever the invitation
// parameter is present, without first deciding whether the token resolves, so
// "this handle maps to nothing" is the ordinary path for a dead or forged
// invitation — not a fault. The caller answers it the same way it answers an
// uninvited dance, which is what keeps /start from being an oracle.
//
// GETDEL, not GET-then-DEL, for the reason ConsumeCode documents: the single
// round trip is what makes the handle genuinely one-shot. A handle that
// survives its read stays live for the rest of its window, and redeeming it is
// what unlocks account creation on an invite-only instance.
func (s *Store) ConsumeInvitationHandle(ctx context.Context, handle string) (*uuid.UUID, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.OAuthDance.ConsumeInvitationHandle")
	defer span.End()

	raw, err := s.db.GetDel(ctx, invitationKey(handle)).Result()
	if err != nil {
		if errors.Is(err, valkeylib.Nil) {
			return nil, nil //nolint:nilnil // "no handle to redeem" is not an error condition here; see the doc comment.
		}

		return nil, fmt.Errorf("consume invitation handle: %w", err)
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		// Only this service writes these values, so an unparseable one means the
		// entry was corrupted or tampered with. Refusing loudly is right: the id
		// decides which invitation's roles are granted.
		return nil, fmt.Errorf("parse invitation id from handle: %w", err)
	}

	return &id, nil
}
