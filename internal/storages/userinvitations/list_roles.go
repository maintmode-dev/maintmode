package userinvitations

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// ListPendingRoles returns the role set of every live pending invitation
// (status = pending and not yet expired) — one entry per invitation. It feeds
// the pending half of the license seat report; expired-but-not-yet-revoked
// rows do not hold a seat.
func (s *Store) ListPendingRoles(ctx context.Context) ([][]entity.Role, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.ListPendingRoles")
	defer span.End()

	stmt := table.UserInvitations.
		SELECT(table.UserInvitations.Roles).
		WHERE(
			table.UserInvitations.Status.EQ(postgres.String(string(entity.InvitationStatusPending))).
				AND(table.UserInvitations.ExpiresAt.GT_EQ(postgres.TimestampzT(xtime.UTCNow()))),
		)

	var rows []model.UserInvitations
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	return lo.Map(rows, func(r model.UserInvitations, _ int) []entity.Role {
		return lo.Map(r.Roles, func(role string, _ int) entity.Role { return entity.Role(role) })
	}), nil
}
