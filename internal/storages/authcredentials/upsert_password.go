package authcredentials

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// UpsertPassword sets the user's password hash, creating the credential or
// replacing the existing one in a single statement.
//
// The conflict target repeats the partial index's predicate -- ON CONFLICT
// (user_id) WHERE kind = 'password' -- because that is what makes Postgres
// infer auth_credentials_password_uidx rather than looking for a total index on
// user_id and failing. The inserted row sets kind for the same reason: a row
// that does not satisfy the predicate is not covered by the index and would
// never conflict.
//
// The DO UPDATE clears the code-shaped columns. They are meaningless for a
// password, and a row that previously held a one-time code keeps its state
// otherwise: a stale attempts count or a consumed_at on a live password is
// state nothing reads and nothing clears.
//
// Replacing in place is deliberate. Retiring an old password by setting
// consumed_at would leave the row occupying the partial unique index and still
// visible to GetPasswordByUserID, since neither filters on it -- a password
// that reads as live but is meant to be dead.
func (s *Store) UpsertPassword(ctx context.Context, userID uuid.UUID, phc string) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthCredentials.UpsertPassword")
	defer span.End()

	// The predicate must match the index's DEFINITION textually enough for
	// Postgres to infer it. postgres.String() renders a $1::text cast, and
	// `kind = 'password'::text` does not match `WHERE kind = 'password'` --
	// inference fails with 42P10 at runtime, not at build time. A raw expression
	// keeps the literal bare.
	isPassword := postgres.RawBool("kind = 'password'")

	stmt := table.AuthCredentials.
		INSERT(
			table.AuthCredentials.UserID,
			table.AuthCredentials.Kind,
			table.AuthCredentials.SecretHash,
		).
		MODEL(&model.AuthCredentials{
			UserID:     userID,
			Kind:       string(entity.AuthCredentialKindPassword),
			SecretHash: phc,
		}).
		ON_CONFLICT(table.AuthCredentials.UserID).
		WHERE(isPassword).
		DO_UPDATE(postgres.SET(
			table.AuthCredentials.SecretHash.SET(postgres.String(phc)),
			table.AuthCredentials.UpdatedAt.SET(postgres.NOW()),
			table.AuthCredentials.Attempts.SET(postgres.Int(0)),
			// Nulled through CAST rather than TimestampzExp(postgres.NULL).
			//
			// SET is typed -- a Timestampz column takes a TimestampzExpression --
			// while postgres.NULL is a plain Expression, so it needs converting
			// either way. Doing that with the bare wrapper is what races: the
			// wrapper calls setRoot on the expression it is handed, and
			// postgres.NULL is ONE package-level value shared by the whole
			// process. Two concurrent password writes then mutate the same
			// object -- `go test -race` reports it, and it is real in
			// production, where two people changing their passwords at the same
			// moment take this path.
			//
			// CAST wraps NULL in a fresh expression first, so the mutation lands
			// on something this statement owns. The generated SQL carries an
			// explicit ::timestamp with time zone, which is what a bare NULL
			// would have been inferred as anyway.
			table.AuthCredentials.ExpiresAt.SET(postgres.CAST(postgres.NULL).AS_TIMESTAMPZ()),
			table.AuthCredentials.ConsumedAt.SET(postgres.CAST(postgres.NULL).AS_TIMESTAMPZ()),
			table.AuthCredentials.SessionNonce.SET(postgres.CAST(postgres.NULL).AS_TEXT()),
		))

	if _, err := stmt.ExecContext(ctx, s.db.Executor(ctx)); err != nil {
		// The raw driver error is not wrapped into the message: a unique
		// violation on this table carries the offending secret_hash in its
		// DETAIL field, and this column holds credential hashes.
		return err
	}

	return nil
}
