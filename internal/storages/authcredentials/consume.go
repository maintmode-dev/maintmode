package authcredentials

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// ConsumeOTP marks a one-time code as used and reports whether this call is the
// one that claimed it. The guard lives in the WHERE clause, which is what makes
// consumption single-use under concurrency: of two callers racing for the same
// row, exactly one updates a row and gets true, while the loser matches nothing
// and gets false.
//
// The active-otp partial unique index is not enough on its own -- it keeps a
// user to one live code, but says nothing about two callers consuming that one
// code simultaneously. Taking a row lock instead would force the caller into a
// transaction it does not otherwise need.
//
// Expiry is deliberately not checked here: the caller decides what counts as
// fresh before calling, the same way invitation acceptance does. This guards
// single use, not freshness.
//
// The id comes from a prior read, so the guard stays on the exact row whose
// secret was verified.
func (s *Store) ConsumeOTP(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthCredentials.ConsumeOTP")
	defer span.End()

	now := xtime.UTCNow()

	stmt := table.AuthCredentials.
		UPDATE(
			table.AuthCredentials.ConsumedAt,
			table.AuthCredentials.UpdatedAt,
		).
		SET(
			postgres.TimestampzT(now),
			postgres.TimestampzT(now),
		).
		WHERE(
			table.AuthCredentials.ID.EQ(postgres.UUID(id)).
				AND(table.AuthCredentials.Kind.EQ(
					postgres.String(string(entity.AuthCredentialKindOTP)),
				)).
				AND(table.AuthCredentials.ConsumedAt.IS_NULL()),
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
