package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/table"
)

func (s *Store) Get(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
	ctx = xlog.WithOperation(ctx, "store.Maintenances.Get")

	stmt := table.Maintenances.
		SELECT(table.Maintenances.AllColumns).
		WHERE(table.Maintenances.ID.EQ(postgres.UUID(maintID)))

	maint := new(model.Maintenances)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), maint)
	if err != nil {
		return nil, err
	}

	return fromDBMaintenance(maint), nil
}

func (s *Store) GetForUpdate(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
	ctx = xlog.WithOperation(ctx, "store.Maintenances.GetForUpdate")

	stmt := table.Maintenances.
		SELECT(table.Maintenances.AllColumns).
		WHERE(table.Maintenances.ID.EQ(postgres.UUID(maintID))).
		FOR(postgres.UPDATE())

	maint := new(model.Maintenances)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), maint)
	if err != nil {
		return nil, err
	}

	return fromDBMaintenance(maint), nil
}
