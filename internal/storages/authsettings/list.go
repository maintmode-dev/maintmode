package authsettings

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// List returns every built-in method flag, ordered by method for a stable read.
func (s *Store) List(ctx context.Context) ([]*entity.AuthMethodSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthSettings.List")
	defer span.End()

	stmt := table.AuthSettings.
		SELECT(table.AuthSettings.AllColumns).
		ORDER_BY(table.AuthSettings.Method.ASC())

	return s.query(ctx, stmt)
}

// query runs a select and maps the rows.
func (s *Store) query(ctx context.Context, stmt postgres.SelectStatement) ([]*entity.AuthMethodSetting, error) {
	var rows []model.AuthSettings
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	out := make([]*entity.AuthMethodSetting, 0, len(rows))
	for i := range rows {
		out = append(out, fromDB(&rows[i]))
	}

	return out, nil
}
