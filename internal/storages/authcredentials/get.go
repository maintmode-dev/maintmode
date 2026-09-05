package authcredentials

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// GetUnconsumedOTPByUserID returns the user's unconsumed one-time code, if any.
// The active-otp partial unique index guarantees there is at most one, so this
// cannot match more than a single row.
//
// Unconsumed is not the same as valid: expires_at is NOT checked here, and an
// expired code comes back like any other. The caller must compare ExpiresAt
// against now before treating the result as usable -- neither this store nor
// the schema does it. The name says "unconsumed" rather than "active" precisely
// so that obligation is visible at the call site: a short lifetime is the main
// control on a code delivered by email, and silently losing it would leave a
// code that never stops working.
func (s *Store) GetUnconsumedOTPByUserID(ctx context.Context, userID uuid.UUID) (*entity.AuthCredential, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthCredentials.GetUnconsumedOTPByUserID")
	defer span.End()

	stmt := table.AuthCredentials.
		SELECT(table.AuthCredentials.AllColumns).
		WHERE(
			table.AuthCredentials.UserID.EQ(postgres.UUID(userID)).
				AND(table.AuthCredentials.Kind.EQ(
					postgres.String(string(entity.AuthCredentialKindOTP)),
				)).
				AND(table.AuthCredentials.ConsumedAt.IS_NULL()),
		)

	return s.get(ctx, stmt)
}

// GetUnconsumedOTPByUserIDForUpdate is GetUnconsumedOTPByUserID holding a row
// lock, for the caller that decides whether to retire the code it reads.
//
// The lock is what makes the "burnt codes keep the slot" rule hold. Reissue
// reads the live code, decides it is not yet burnt, and consumes it; a verify
// racing that read can be claiming the final attempt at the same moment. Without
// the lock the reader sees attempts one short of the ceiling, judges the code
// still usable, and frees the slot -- handing back a fresh code with a fresh
// counter, which is exactly the bypass the ceiling exists to prevent.
//
// ConsumeOTP deliberately takes no lock because its caller has no transaction
// and needs none; this one is always called inside the issuance transaction, so
// the lock costs it nothing beyond the hold it already has.
func (s *Store) GetUnconsumedOTPByUserIDForUpdate(
	ctx context.Context,
	userID uuid.UUID,
) (*entity.AuthCredential, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthCredentials.GetUnconsumedOTPByUserIDForUpdate")
	defer span.End()

	stmt := table.AuthCredentials.
		SELECT(table.AuthCredentials.AllColumns).
		WHERE(
			table.AuthCredentials.UserID.EQ(postgres.UUID(userID)).
				AND(table.AuthCredentials.Kind.EQ(
					postgres.String(string(entity.AuthCredentialKindOTP)),
				)).
				AND(table.AuthCredentials.ConsumedAt.IS_NULL()),
		).
		FOR(postgres.UPDATE())

	return s.get(ctx, stmt)
}

// GetPasswordByUserID returns the user's password credential, if any. The
// password partial unique index guarantees there is at most one.
func (s *Store) GetPasswordByUserID(ctx context.Context, userID uuid.UUID) (*entity.AuthCredential, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthCredentials.GetPasswordByUserID")
	defer span.End()

	stmt := table.AuthCredentials.
		SELECT(table.AuthCredentials.AllColumns).
		WHERE(
			table.AuthCredentials.UserID.EQ(postgres.UUID(userID)).
				AND(table.AuthCredentials.Kind.EQ(
					postgres.String(string(entity.AuthCredentialKindPassword)),
				)),
		)

	return s.get(ctx, stmt)
}

func (s *Store) get(ctx context.Context, stmt postgres.Statement) (*entity.AuthCredential, error) {
	row := new(model.AuthCredentials)

	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), row); err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrAuthCredentialNotFound
		}
		return nil, err
	}

	return fromDB(row), nil
}
