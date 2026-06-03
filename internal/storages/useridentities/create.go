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
		// Either unique index (provider, subject) or (user_id, provider) was
		// violated — the identity is already linked. Translate to a domain
		// error so callers don't depend on the pq driver.
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrProviderAlreadyConnected
		}
		return nil, err
	}

	return fromDB(row), nil
}
