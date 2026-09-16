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

// GetByKindName reads one integration by its identity. Not-found is
// ErrIntegrationNotFound. This is the resolver's read path.
//
// The name is half the identity, not an optional refinement: since the schema
// allows several rows per kind, a kind-only lookup would return an arbitrary
// one. The method is named for both halves so a caller that has only a kind
// cannot reach it by accident.
func (s *Store) GetByKindName(ctx context.Context, kind, name string) (*entity.IntegrationSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Integration.GetByKindName")
	defer span.End()

	stmt := table.IntegrationSettings.
		SELECT(table.IntegrationSettings.AllColumns).
		WHERE(table.IntegrationSettings.Kind.EQ(postgres.String(kind)).
			AND(table.IntegrationSettings.Name.EQ(postgres.String(name))))

	return s.get(ctx, stmt)
}

// GetForUpdateByKindName reads and locks the integration row (FOR UPDATE) so a
// read-modify-write update is serialized. Must run inside a transaction.
func (s *Store) GetForUpdateByKindName(ctx context.Context, kind, name string) (*entity.IntegrationSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Integration.GetForUpdateByKindName")
	defer span.End()

	stmt := table.IntegrationSettings.
		SELECT(table.IntegrationSettings.AllColumns).
		WHERE(table.IntegrationSettings.Kind.EQ(postgres.String(kind)).
			AND(table.IntegrationSettings.Name.EQ(postgres.String(name)))).
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
