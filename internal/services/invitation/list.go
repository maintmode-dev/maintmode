package invitation

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// List returns admin-view invitations filtered by status (empty = all). This is
// the only place invitation contents (email, roles, inviter) are exposed, and
// it is admin-only.
func (s *Service) List(ctx context.Context, cmd *entity.ListInvitationsCmd) ([]*entity.InvitationListItem, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.List")
	defer span.End()

	items, err := s.store.List(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "list invitations failed", xfield.Error(err))
		return nil, err
	}

	return items, nil
}
