package userinvitations

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// ExpireOlderThan flips up to limit pending invitations whose expires_at is
// strictly before now to the persisted 'expired' status, returning how many it
// changed. It is the single batch behind the rotation sweep: the caller loops
// until a batch changes fewer than limit rows.
//
// Postgres has no UPDATE ... LIMIT, so the batch is bounded by an id-subquery:
// pick the oldest `limit` eligible ids (ORDER BY expires_at ASC), then update
// exactly those. Bounding each batch keeps the per-statement lock footprint small.
//
// There is intentionally no index on expires_at: the table is small (a few
// thousand rows) so the daily off-hours sweep seq-scans + top-N sorts in a few
// ms. Add a (status, expires_at) index if invitations ever become high-volume.
//
// Only pending rows are eligible, so a rotated row leaves the partial-unique
// pending index and frees the email's active-pending slot. The update cannot
// collide on that index (it moves rows out of it), so the raw store error is
// returned without apperr translation.
func (s *Store) ExpireOlderThan(ctx context.Context, now time.Time, limit int64) (int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.ExpireOlderThan")
	defer span.End()

	stale := table.UserInvitations.
		SELECT(table.UserInvitations.ID).
		WHERE(
			table.UserInvitations.Status.EQ(postgres.String(string(entity.InvitationStatusPending))).
				AND(table.UserInvitations.ExpiresAt.LT(postgres.TimestampzT(now))),
		).
		ORDER_BY(table.UserInvitations.ExpiresAt.ASC()).
		LIMIT(limit)

	stmt := table.UserInvitations.
		UPDATE(table.UserInvitations.Status).
		SET(postgres.String(string(entity.InvitationStatusExpired))).
		WHERE(table.UserInvitations.ID.IN(stale))

	res, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
