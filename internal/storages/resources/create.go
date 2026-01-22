package resources

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/table"
)

func (s *Store) Create(ctx context.Context, resource *model.Resources) error {
	ctx = xlog.WithOperation(ctx, "store.Resources.Create")

	resource.ID = xuuid.New()

	stmt := table.Resources.
		INSERT(table.Resources.AllColumns).
		MODEL(resource).
		RETURNING(table.Resources.AllColumns)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), resource)
	if err != nil {
		return err
	}
	return nil
}
