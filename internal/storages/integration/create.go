package integration

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Create inserts a new integration and returns the stored row. A duplicate kind
// (UNIQUE(kind)) surfaces as ErrIntegrationConflict. id/created_at/updated_at are
// assigned by the database.
func (s *Store) Create(ctx context.Context, setting *entity.IntegrationSetting) (*entity.IntegrationSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Integration.Create")
	defer span.End()

	dbModel, err := toDB(setting)
	if err != nil {
		return nil, err
	}

	stmt := table.IntegrationSettings.
		INSERT(
			table.IntegrationSettings.MutableColumns.
				Except(table.IntegrationSettings.CreatedAt).
				Except(table.IntegrationSettings.UpdatedAt),
		).
		MODEL(dbModel).
		RETURNING(table.IntegrationSettings.AllColumns)

	inserted := new(model.IntegrationSettings)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), inserted); err != nil {
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrIntegrationConflict
		}
		xlog.Error(ctx, "create integration failed", xfield.Error(err))
		return nil, err
	}

	return fromDB(inserted)
}
