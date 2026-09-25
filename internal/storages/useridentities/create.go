package useridentities

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

func (s *Store) Create(ctx context.Context, identity *entity.UserIdentity) (*entity.UserIdentity, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.Create")
	defer span.End()

	row := toDB(identity)

	stmt := table.UserIdentities.
		INSERT(table.UserIdentities.MutableColumns.
			Except(table.UserIdentities.CreatedAt),
		).
		MODEL(row).
		RETURNING(table.UserIdentities.AllColumns)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), row)
	if err != nil {
		// One of the four partial unique indexes was violated -- by subject
		// (user_identities_integration_subject_uidx,
		// user_identities_builtin_subject_uidx) or by user and method
		// (user_identities_user_integration_uidx,
		// user_identities_user_builtin_uidx). All four say the same thing here:
		// the identity is already linked. Translate to a domain error so callers
		// don't depend on the pq driver.
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrProviderAlreadyConnected
		}
		return nil, err
	}

	return fromDB(row), nil
}
