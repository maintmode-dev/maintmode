package resources

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/table"
)

func (s *Store) GetByID(ctx context.Context, resourceID uuid.UUID) (*model.Resources, error) {
	ctx = xlog.WithOperation(ctx, "store.Resources.GetByID")

	stmt := table.Resources.
		SELECT(table.Resources.AllColumns).
		WHERE(table.Resources.ID.EQ(postgres.UUID(resourceID)))

	resource := new(model.Resources)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), resource)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *Store) GetByName(ctx context.Context, name string) (*model.Resources, error) {
	ctx = xlog.WithOperation(ctx, "store.Resources.GetByName")

	stmt := table.Resources.
		SELECT(table.Resources.AllColumns).
		WHERE(table.Resources.Name.EQ(postgres.String(name)))

	resource := new(model.Resources)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), resource)
	if err != nil {
		return nil, err
	}
	return resource, nil
}
