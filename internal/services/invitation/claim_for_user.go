package invitation

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// claimAndAssignRoles spends an invitation and grants its roles, atomically.
//
// It is the ONE copy of that transaction. Both accept paths call it — the
// deprecated id_token Accept and the invited dance's ClaimForUser — because the
// ordering below is load-bearing and invisible to the compiler, and a second
// copy is how an invariant like that gets fixed in one place and not the other.
// It returns the updated user because Accept still needs it to issue a token
// pair; the dance discards it.
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
func (s *Service) claimAndAssignRoles(
	ctx context.Context,
	invitationID uuid.UUID,
	userID uuid.UUID,
	roles []entity.Role,
) (*entity.User, error) {
	var user *entity.User

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		claimed, err := s.store.MarkAccepted(ctx, invitationID)
		if err != nil {
			return fmt.Errorf("mark accepted: %w", err)
		}
		if !claimed {
			// Someone else took it, or it stopped being pending since phase 0.
			return apperr.ErrInvalidInvitation
		}

		user, err = s.userSrv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  entity.SystemUser,
			UserID: userID,
			Roles:  roles,
		})
		if err != nil {
			return fmt.Errorf("assign invitation roles: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return user, nil
}

// ClaimForUser is phase 2 of an invited dance: it spends the invitation and
// grants its roles to the user the dance just signed in.
//
// The transaction, and the ordering invariant it rests on, live in
// claimAndAssignRoles, which the deprecated id_token accept path shares.
func (s *Service) ClaimForUser(ctx context.Context, inv *entity.ResolvedInvitation, userID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.ClaimForUser")
	defer span.End()

	if _, err := s.claimAndAssignRoles(ctx, inv.ID, userID, inv.Roles); err != nil {
		xlog.Error(ctx, "claim invitation: claim and assign roles failed", xfield.Error(err))

		return err
	}

	return nil
}
