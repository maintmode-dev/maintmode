package integration

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Update persists the editable columns of an integration and returns the stored
// row. The caller passes a fully merged entity (read-modify-write). Only
// enabled/config/secrets/dek_id and the updated_* authorship are written;
// kind/created_* are left untouched so the type key and original author survive.
//
// updated_at is stamped here (not by the caller) to the app clock, matching the
// repo's other stores (resources/notifychannel/maintenances) — the store owns the
// modification timestamp so every write advances it.
func (s *Store) Update(ctx context.Context, setting *entity.IntegrationSetting) (*entity.IntegrationSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Integration.Update")
	defer span.End()

	setting.UpdatedAt = xtime.UTCNow()

	dbModel, err := toDB(setting)
	if err != nil {
		return nil, err
	}

	stmt := table.IntegrationSettings.
		UPDATE(
			table.IntegrationSettings.Enabled,
			table.IntegrationSettings.Config,
			table.IntegrationSettings.Secrets,
			table.IntegrationSettings.DekID,
			table.IntegrationSettings.UpdatedAt,
			table.IntegrationSettings.UpdatedByUserID,
		).
		MODEL(dbModel).
		WHERE(table.IntegrationSettings.ID.EQ(postgres.UUID(setting.ID))).
		RETURNING(table.IntegrationSettings.AllColumns)

	updated := new(model.IntegrationSettings)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), updated); err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrIntegrationNotFound
		}
		return nil, err
	}

	return fromDB(updated)
}
