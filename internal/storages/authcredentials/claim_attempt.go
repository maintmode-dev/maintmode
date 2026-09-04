package authcredentials

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// ClaimOTPAttempt spends one attempt against a live one-time code and reports
// whether the ceiling still had room for it.
//
// The ceiling lives in the WHERE clause, and that is the whole point of the
// method existing at all. A read, a `attempts < max` check in Go, and a separate
// increment would cap RECORDED attempts rather than PERFORMED ones: N callers
// racing on the same code all read the same stale count, all pass the check, and
// all go on to compare their guess against the secret. The counter still lands
// on max, so nothing looks wrong afterwards -- the attacker simply got N guesses
// instead of max. Here the database arbitrates, so exactly max callers are ever
// told to proceed.
//
// It follows that the claim happens BEFORE the comparison it pays for. A caller
// that claims and then fails to compare has burned an attempt for nothing; that
// is the deliberate direction to fail in, since the alternative leaves a window
// where guesses are free.
//
// The kind and consumed_at conjuncts mirror ConsumeOTP's. Without the first this
// statement increments a password row -- which also has consumed_at NULL and
// attempts 0 -- and reports success, invisibly, because the password getter
// filters on neither column.
func (s *Store) ClaimOTPAttempt(ctx context.Context, id uuid.UUID, maxAttempts int16) (bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthCredentials.ClaimOTPAttempt")
	defer span.End()

	stmt := table.AuthCredentials.
		UPDATE(
			table.AuthCredentials.Attempts,
			table.AuthCredentials.UpdatedAt,
		).
		SET(
			table.AuthCredentials.Attempts.ADD(postgres.Int16(1)),
			postgres.TimestampzT(xtime.UTCNow()),
		).
		WHERE(
			table.AuthCredentials.ID.EQ(postgres.UUID(id)).
				AND(table.AuthCredentials.Kind.EQ(
					postgres.String(string(entity.AuthCredentialKindOTP)),
				)).
				AND(table.AuthCredentials.ConsumedAt.IS_NULL()).
				AND(table.AuthCredentials.Attempts.LT(postgres.Int16(maxAttempts))),
		).
		RETURNING(table.AuthCredentials.Attempts)

	row := new(model.AuthCredentials)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), row); err != nil {
		// No row updated: the ceiling is spent, or this is not a live code. Both
		// mean the same thing to the caller -- no guess was bought -- so they are
		// not distinguished here. The caller derives its audit reason from the
		// false, not from a count.
		if errors.Is(err, qrm.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
