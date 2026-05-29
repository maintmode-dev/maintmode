package resources

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) Create(ctx context.Context, r *entity.ResourceDetails) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Resources.Create")
	defer span.End()

	resource := toDBResource(r)

	stmt := table.Resources.
		INSERT(table.Resources.MutableColumns.
			Except(table.Resources.CreatedAt),
		).
		MODEL(resource).
		RETURNING(table.Resources.AllColumns)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), resource)
	if err != nil {
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrResourceAlreadyExists
		}
		return nil, err
	}
	return fromDBResource(resource), nil
}
