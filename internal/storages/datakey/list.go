package datakey

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// ListForUpdate returns every DEK row with FOR UPDATE row locks. Rotation calls
// this inside a transaction so no concurrent writer can re-wrap the same rows
// while a rotation is re-wrapping them; the locks release on commit/rollback.
func (s *Store) ListForUpdate(ctx context.Context) ([]*entity.DataKey, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.DataKeys.ListForUpdate")
	defer span.End()

	stmt := table.DataKeys.
		SELECT(table.DataKeys.AllColumns).
		FOR(postgres.UPDATE())

	var rows []model.DataKeys
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		xlog.Error(ctx, "list data keys for update failed", xfield.Error(err))
		return nil, err
	}

	return fromDBList(rows), nil
}
