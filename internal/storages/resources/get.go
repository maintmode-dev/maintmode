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
	ctx = xlog.WithOperation(ctx, "store.Resources.GetByID")

	stmt := table.Resources.
		SELECT(table.Resources.AllColumns).
		WHERE(table.Resources.ID.EQ(postgres.UUID(resourceID)))

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

func (s *Store) GetByName(ctx context.Context, name string) (*entity.ResourceDetails, error) {
	ctx = xlog.WithOperation(ctx, "store.Resources.GetByName")

	stmt := table.Resources.
		SELECT(table.Resources.AllColumns).
		WHERE(table.Resources.Name.EQ(postgres.String(name)))

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
