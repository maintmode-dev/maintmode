package claimer

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// ClaimForUser spends an invitation and grants its roles, atomically, to a user
// who already exists.
//
// It is the ONE copy of that transaction. Both accept paths call it — the
// deprecated id_token Accept and phase 2 of the invited dance — because the
// ordering below is load-bearing and invisible to the compiler, and a second
// copy is how an invariant like that gets fixed in one place and not the other.
// It returns the updated user because Accept still needs it to issue a token
// pair carrying the granted roles; the dance discards it.
//
// Both halves run in ONE transaction so an accepted invitation can never be
// left with a user missing its roles: if AssignRoles fails, the claim rolls back
// and the link stays usable.
//
// MarkAccepted is the single-use gate. Of two concurrent claims of the same
// invitation exactly one flips pending→accepted; the loser matches zero rows and
// is rejected. The dance's own state signature provides no uniqueness — it is
// deterministic and re-verifies for its whole window — so this is the only thing
// preventing one link from onboarding N accounts.
//
// ORDER IS LOAD-BEARING: MarkAccepted must precede AssignRoles.
//
// AssignRoles fires the seats guard on a non-seat→seat transition, and that
// guard counts live PENDING invitations as occupied seats. Reads inside a
// transaction see its own uncommitted writes, so once MarkAccepted has flipped
// the row the invitation has left the pending set: its reservation is released,
// and the guard's occupied+1 re-adds exactly the same person. Net zero, and the
// invitee keeps the seat that was reserved for them at invite time.
//
// Swap the two — assign first, then mark — and the invitation is still pending
// when the guard counts. It then counts this invitee once as a pending invite
// and once more as the grant in flight, and at exactly the cap it refuses them
// their own seat. The failure looks like a licensing problem and is not.
func (c *Claimer) ClaimForUser(
	ctx context.Context,
	inv *entity.ResolvedInvitation,
	userID uuid.UUID,
) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.InvitationClaimer.ClaimForUser")
	defer span.End()

	var user *entity.User

	err := c.txManager.WithinTx(ctx, func(ctx context.Context) error {
		claimed, err := c.store.MarkAccepted(ctx, inv.ID)
		if err != nil {
			return fmt.Errorf("mark accepted: %w", err)
		}
		if !claimed {
			// Someone else took it, or it stopped being pending since phase 0.
			return apperr.ErrInvalidInvitation
		}

		user, err = c.userSrv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  entity.SystemUser,
			UserID: userID,
			Roles:  inv.Roles,
		})
		if err != nil {
			return fmt.Errorf("assign invitation roles: %w", err)
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "claim invitation: claim and assign roles failed", xfield.Error(err))

		return nil, err
	}

	return user, nil
}
