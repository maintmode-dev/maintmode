package refreshtoken

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// PruneExpiredBefore deletes up to limit refresh-token rows whose expires_at is
// strictly before cutoff, returning how many it removed. It is the single batch
// behind the retention sweep: the caller loops until a batch deletes fewer than
// limit.
//
// Revocation state is deliberately absent from the predicate. A revoked row is
// kept as long as a live one, because reuse detection needs it: a rotated-out
// token replayed while its family is still alive must be found to revoke that
// family, and a deleted row would answer "unknown token" instead. Once
// expires_at has passed the whole family is past its maximum lifetime (config
// validation keeps refresh_token_ttl at or above session_max_lifetime), so
// nothing the row could still decide remains.
//
// Postgres has no DELETE ... LIMIT, so the batch is bounded by an id-subquery
// ordered by idx_refresh_tokens_expires_at: pick the oldest `limit` eligible
// hashes, then delete exactly those.
func (s *Store) PruneExpiredBefore(ctx context.Context, cutoff time.Time, limit int64) (int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.PruneExpiredBefore")
	defer span.End()

	expired := table.RefreshTokens.
		SELECT(table.RefreshTokens.TokenHash).
		WHERE(table.RefreshTokens.ExpiresAt.LT(postgres.TimestampzT(cutoff))).
		ORDER_BY(table.RefreshTokens.ExpiresAt.ASC()).
		LIMIT(limit)

	stmt := table.RefreshTokens.
		DELETE().
		WHERE(table.RefreshTokens.TokenHash.IN(expired))

	res, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
