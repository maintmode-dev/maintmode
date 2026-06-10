package userinvitations

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// listRow is the join destination. The embedded UserInvitations soaks up the
// invitation's own columns; the inviter's resolved profile arrives under the
// "inviter.*" aliases, which qrm matches via the `alias` struct tag (not `db`).
type listRow struct {
	model.UserInvitations
	InviterName  string `alias:"inviter.name"`
	InviterEmail string `alias:"inviter.email"`
}

// List returns admin-view invitations (with the inviter's profile) filtered by
// status, ordered by sent_at DESC with id DESC as a stable tie-breaker.
//
// The "expired" filter is derived: status='pending' AND expires_at < now. The
// "pending" filter is the live complement: status='pending' AND expires_at >=
// now. accepted/revoked filter on the stored status directly.
func (s *Store) List(ctx context.Context, cmd *entity.ListInvitationsCmd) ([]*entity.InvitationListItem, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.List")
	defer span.End()

	stmt := table.UserInvitations.
		INNER_JOIN(table.Users, table.Users.ID.EQ(table.UserInvitations.InvitedByID)).
		SELECT(
			table.UserInvitations.AllColumns,
			table.Users.Name.AS("inviter.name"),
			table.Users.Email.AS("inviter.email"),
		).
		WHERE(listWhereExpr(cmd)).
		ORDER_BY(
			table.UserInvitations.SentAt.DESC(),
			table.UserInvitations.ID.DESC(),
		)

	rows := make([]*listRow, 0)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	return lo.Map(rows, func(r *listRow, _ int) *entity.InvitationListItem {
		inv := r.UserInvitations
		return &entity.InvitationListItem{
			Invitation: fromDB(&inv),
			Inviter: &entity.UserSummary{
				ID:    inv.InvitedByID,
				Name:  r.InviterName,
				Email: r.InviterEmail,
			},
		}
	}), nil
}

func listWhereExpr(cmd *entity.ListInvitationsCmd) postgres.BoolExpression {
	now := postgres.TimestampzT(xtime.UTCNow())
	pending := table.UserInvitations.Status.EQ(postgres.String(string(entity.InvitationStatusPending)))

	switch cmd.Status {
	case entity.InvitationStatusPending:
		return pending.AND(table.UserInvitations.ExpiresAt.GT_EQ(now))
	case entity.InvitationStatusExpired:
		return pending.AND(table.UserInvitations.ExpiresAt.LT(now))
	case entity.InvitationStatusAccepted, entity.InvitationStatusRevoked:
		return table.UserInvitations.Status.EQ(postgres.String(string(cmd.Status)))
	default:
		return postgres.Bool(true)
	}
}
