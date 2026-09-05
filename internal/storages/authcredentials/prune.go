package authcredentials

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// PruneOTPExpiredBefore deletes up to limit one-time codes whose expires_at is
// strictly before cutoff, returning how many it removed. It is the single batch
// behind the retention sweep: the caller loops until a batch deletes fewer than
// limit.
//
// Age is measured on expires_at, not created_at. auth_credentials_otp_expiry_idx
// is the only index on this table and it is on expires_at; for a one-time code
// the two columns differ by exactly the TTL, so nothing is lost by preferring
// the indexed one.
//
// consumed_at is deliberately absent from the predicate. A consumed code still
// carries an expiry and ages out through this same sweep, so no second branch
// (and no second tunable) is needed to collect it.
//
// The `kind = 'otp'` conjunct is load-bearing twice, and must not be "simplified"
// away on the grounds that password rows have a NULL expiry:
//
//   - Correctness. Nothing in the schema ties kind='password' to a NULL
//     expires_at — there is no such CHECK — so the NULL barrier is an observation
//     about today's write paths, not an invariant. A password row that ever
//     gained an expiry would be deleted by a predicate resting on NULL alone.
//   - The query plan. The index is PARTIAL (WHERE kind = 'otp'), and Postgres can
//     only use it if the query's predicate implies the index's. Drop this literal
//     and the sweep silently degrades to a sequential scan plus sort. No test can
//     observe that, which is why it is written down here.
//
// ORDER BY expires_at ASC matches the index's own direction, so it is served
// directly rather than by a backward scan.
//
// Postgres has no DELETE ... LIMIT, so the batch is bounded by an id-subquery:
// pick the oldest `limit` eligible ids, then delete exactly those. Bounding each
// batch keeps the per-statement lock footprint small on a table that takes a row
// per sign-in attempt.
func (s *Store) PruneOTPExpiredBefore(ctx context.Context, cutoff time.Time, limit int64) (int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthCredentials.PruneOTPExpiredBefore")
	defer span.End()

	expired := table.AuthCredentials.
		SELECT(table.AuthCredentials.ID).
		WHERE(
			table.AuthCredentials.Kind.EQ(postgres.String(string(entity.AuthCredentialKindOTP))).
				AND(table.AuthCredentials.ExpiresAt.LT(postgres.TimestampzT(cutoff))),
		).
		ORDER_BY(table.AuthCredentials.ExpiresAt.ASC()).
		LIMIT(limit)

	stmt := table.AuthCredentials.
		DELETE().
		WHERE(table.AuthCredentials.ID.IN(expired))

	res, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
