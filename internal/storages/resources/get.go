package resources

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

func (s *Store) GetByID(ctx context.Context, resourceID uuid.UUID) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Resources.GetByID")
	defer span.End()

	stmt := table.Resources.
		SELECT(table.Resources.AllColumns).
		WHERE(table.Resources.ID.EQ(postgres.UUID(resourceID)))

	return s.get(ctx, stmt)
}

func (s *Store) GetByName(ctx context.Context, name string) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Resources.GetByName")
	defer span.End()

	stmt := table.Resources.
		SELECT(table.Resources.AllColumns).
		WHERE(postgres.LOWER(table.Resources.Name).EQ(postgres.LOWER(postgres.String(name))))

	return s.get(ctx, stmt)
}

func (s *Store) get(ctx context.Context, stmt postgres.SelectStatement) (*entity.ResourceDetails, error) {
	resource := new(model.Resources)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), resource)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrResourceNotFound
		}
		return nil, err
	}
	return fromDBResource(resource), nil
}
