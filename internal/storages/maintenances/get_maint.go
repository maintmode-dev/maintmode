package maintenances

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

func (s *Store) GetMaint(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.GetMaint")
	defer span.End()

	stmt := table.Maintenances.
		SELECT(table.Maintenances.AllColumns).
		WHERE(table.Maintenances.ID.EQ(postgres.UUID(maintID)))

	return s.getMaint(ctx, stmt)
}

func (s *Store) GetMaintForUpdate(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.GetMaintForUpdate")
	defer span.End()

	stmt := table.Maintenances.
		SELECT(table.Maintenances.AllColumns).
		WHERE(table.Maintenances.ID.EQ(postgres.UUID(maintID))).
		FOR(postgres.UPDATE())

	return s.getMaint(ctx, stmt)
}

func (s *Store) getMaint(ctx context.Context, stmt postgres.Statement) (*entity.Maintenance, error) {
	maint := new(model.Maintenances)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), maint)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrMaintNotFound
		}
		return nil, err
	}

	return fromDBMaintenance(maint), nil
}
