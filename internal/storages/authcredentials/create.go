package authcredentials

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Create inserts a credential and returns it as stored.
//
// created_at and updated_at are left to the schema defaults: writing the Go
// zero time over them would put the row in year 1 and make a later
// "updated_at moved" check unreadable.
//
// consumed_at is excluded because consuming is ConsumeOTP's job. Were it
// insertable, a caller passing a non-nil value -- most easily by building the
// entity from one it had just read -- would create a code born dead: it stays
// out of the active-code index so it blocks nothing and conflicts with nothing,
// no read path returns it, and consuming it always reports false. That is the
// same shape of hole the kind constraint closes, reached through a different
// column.
//
// attempts stays insertable, so a caller can seed a row at a chosen count.
func (s *Store) Create(ctx context.Context, cred *entity.AuthCredential) (*entity.AuthCredential, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthCredentials.Create")
	defer span.End()

	row := toDB(cred)

	stmt := table.AuthCredentials.
		INSERT(table.AuthCredentials.MutableColumns.
			Except(
				table.AuthCredentials.ConsumedAt,
				table.AuthCredentials.CreatedAt,
				table.AuthCredentials.UpdatedAt,
			),
		).
		MODEL(row).
		RETURNING(table.AuthCredentials.AllColumns)

	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), row); err != nil {
		// Either partial unique index can raise this: the user already has a
		// password, or already has a live one-time code. One sentinel covers
		// both because a caller knows the kind it was writing and reads the
		// meaning off that. The primary key is a third unique constraint, but
		// it is uuidv7-defaulted and a collision is not a case worth branching
		// on.
		//
		// What the sentinel does not distinguish is why: on a reissue path the
		// caller freed the slot in the same transaction, so a conflict there
		// means a concurrent issuer won the race and the operation is worth
		// retrying, whereas the same error from a first write is a genuine
		// "already exists". The caller has the context to tell those apart.
		//
		// A CHECK violation is deliberately not caught here. dbtx.ErrorIs
		// matches one SQLSTATE, so a bad kind surfaces raw -- that is a
		// programmer error, not something a caller can act on.
		//
		// That raw error is not safe to render freely: Postgres puts the entire
		// failing row into the constraint violation's DETAIL field, secret_hash
		// included, and lib/pq keeps it. Error() prints only the message and the
		// code, so logging or spanning this error the ordinary way is fine;
		// ErrorWithDetail, %+v on the *pq.Error, and any reporter that reflects
		// over the error struct are not.
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrAuthCredentialConflict
		}
		return nil, err
	}

	return fromDB(row), nil
}
