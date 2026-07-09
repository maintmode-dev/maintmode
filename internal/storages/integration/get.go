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
)

// GetByKind reads the integration for a kind. Not-found is ErrIntegrationNotFound.
// This is the resolver's read path.
func (s *Store) GetByKind(ctx context.Context, kind string) (*entity.IntegrationSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Integration.GetByKind")
	defer span.End()

	stmt := table.IntegrationSettings.
		SELECT(table.IntegrationSettings.AllColumns).
		WHERE(table.IntegrationSettings.Kind.EQ(postgres.String(kind)))

	return s.get(ctx, stmt)
}

// GetForUpdateByKind reads and locks the integration row (FOR UPDATE) so a
// read-modify-write update is serialized. Must run inside a transaction.
func (s *Store) GetForUpdateByKind(ctx context.Context, kind string) (*entity.IntegrationSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Integration.GetForUpdateByKind")
	defer span.End()

	stmt := table.IntegrationSettings.
		SELECT(table.IntegrationSettings.AllColumns).
		WHERE(table.IntegrationSettings.Kind.EQ(postgres.String(kind))).
		FOR(postgres.UPDATE())

	return s.get(ctx, stmt)
}

func (s *Store) get(ctx context.Context, stmt postgres.SelectStatement) (*entity.IntegrationSetting, error) {
	dbModel := new(model.IntegrationSettings)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), dbModel); err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrIntegrationNotFound
		}
		return nil, err
	}
	return fromDB(dbModel)
}
