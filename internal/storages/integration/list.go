package integration

import (
	"context"

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
