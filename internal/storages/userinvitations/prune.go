package userinvitations

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// PruneTerminalOlderThan deletes up to limit terminal invitations (expired,
// accepted or revoked) whose created_at is strictly before cutoff, returning how
// many it removed. It is the single batch behind the retention sweep: the caller
// loops until a batch deletes fewer than limit.
//
// pending rows are never eligible — a pending invitation still in play, or one
// past expiry that rotation has not yet flipped, must survive. created_at is the
// age column because a rotated 'expired' row has no accepted_at/revoked_at; at
// the 7-day TTL and a ~1-year retention the created-vs-terminated gap is noise.
//
// Postgres has no DELETE ... LIMIT, so the batch is bounded by an id-subquery:
// pick the oldest `limit` eligible ids (ORDER BY created_at ASC), then delete
// exactly those.
//
// Unlike the audit-prune template, there is intentionally no index on created_at
// here: user_invitations is bounded by admin invite volume (a few thousand rows),
// so the daily off-hours sweep seq-scans + top-N sorts in a few ms. If the table
// ever grows past ~tens of thousands (self-serve/automated invites), add a
// (status, created_at) index to serve this ORDER BY.
func (s *Store) PruneTerminalOlderThan(ctx context.Context, cutoff time.Time, limit int64) (int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.PruneTerminalOlderThan")
	defer span.End()

	stale := table.UserInvitations.
		SELECT(table.UserInvitations.ID).
		WHERE(
			table.UserInvitations.Status.IN(
				postgres.String(string(entity.InvitationStatusExpired)),
				postgres.String(string(entity.InvitationStatusAccepted)),
				postgres.String(string(entity.InvitationStatusRevoked)),
			).
				AND(table.UserInvitations.CreatedAt.LT(postgres.TimestampzT(cutoff))),
		).
		ORDER_BY(table.UserInvitations.CreatedAt.ASC()).
		LIMIT(limit)

	stmt := table.UserInvitations.
		DELETE().
		WHERE(table.UserInvitations.ID.IN(stale))

	res, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
