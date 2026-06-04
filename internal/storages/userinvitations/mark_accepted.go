package userinvitations

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// MarkAccepted atomically transitions a pending invitation to accepted and
// reports whether the transition happened. The status guard in the WHERE clause
// makes acceptance single-use under concurrency: of two racing accepts for the
// same token, exactly one updates a row (returns true); the loser matches zero
// rows (returns false) and the caller rejects it. Expiry is enforced by the
// caller before this call, so a still-pending-but-expired row is not accepted.
func (s *Store) MarkAccepted(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.MarkAccepted")
	defer span.End()

	stmt := table.UserInvitations.
		UPDATE(
			table.UserInvitations.Status,
			table.UserInvitations.AcceptedAt,
		).
		SET(
			postgres.String(string(entity.InvitationStatusAccepted)),
			postgres.TimestampzT(xtime.UTCNow()),
		).
		WHERE(
			table.UserInvitations.ID.EQ(postgres.UUID(id)).
				AND(table.UserInvitations.Status.EQ(
					postgres.String(string(entity.InvitationStatusPending)),
				)),
		)

	res, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
