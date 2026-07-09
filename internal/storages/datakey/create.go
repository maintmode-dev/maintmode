package datakey

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// Create inserts a wrapped DEK and returns the stored row. id and created_at are
// assigned by the database (uuidv7 / now()); the caller supplies kek_id, the
// wrapped bytes, and an optional label.
func (s *Store) Create(ctx context.Context, dk *entity.DataKey) (*entity.DataKey, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.DataKeys.Create")
	defer span.End()

	stmt := table.DataKeys.
		INSERT(
			table.DataKeys.MutableColumns.
				Except(table.DataKeys.CreatedAt),
		).
		MODEL(toDB(dk)).
		RETURNING(table.DataKeys.AllColumns)

	inserted := new(model.DataKeys)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), inserted); err != nil {
		xlog.Error(ctx, "create data key failed", xfield.Error(err))
		return nil, err
	}

	return fromDB(inserted), nil
}
