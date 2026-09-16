package integration

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// List returns all integrations ordered by kind for a stable admin listing.
func (s *Store) List(ctx context.Context) ([]*entity.IntegrationSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Integration.List")
	defer span.End()

	stmt := table.IntegrationSettings.
		SELECT(table.IntegrationSettings.AllColumns).
		ORDER_BY(table.IntegrationSettings.Kind.ASC())

	rows := make([]*model.IntegrationSettings, 0)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	out := make([]*entity.IntegrationSetting, 0, len(rows))
	for _, item := range rows {
		setting, err := fromDB(item)
		if err != nil {
			return nil, err
		}
		out = append(out, setting)
	}
	return out, nil
}

// ListByKind returns every row of one kind, ordered by name for a stable read.
func (s *Store) ListByKind(ctx context.Context, kind string) ([]*entity.IntegrationSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Integration.ListByKind")
	defer span.End()

	stmt := table.IntegrationSettings.
		SELECT(table.IntegrationSettings.AllColumns).
		WHERE(table.IntegrationSettings.Kind.EQ(postgres.String(kind))).
		ORDER_BY(table.IntegrationSettings.Name.ASC())

	var dbModels []model.IntegrationSettings
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &dbModels); err != nil {
		return nil, err
	}

	settings := make([]*entity.IntegrationSetting, 0, len(dbModels))
	for i := range dbModels {
		setting, err := fromDB(&dbModels[i])
		if err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}

	return settings, nil
}
