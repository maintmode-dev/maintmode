package resources

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

const (
	resourceListLimit = 100
)

func (s *Store) GetResources(ctx context.Context, resourceIDs []uuid.UUID) ([]*entity.ResourceDetails, error) {
	ctx = xlog.WithOperation(ctx, "store.Resources.GetResources")

	if len(resourceIDs) == 0 {
		return []*entity.ResourceDetails{}, nil
	}

	stmt := table.Resources.
		SELECT(table.Resources.AllColumns).
		WHERE(table.Resources.ID.EQ(postgres.ANY(
			postgres.ARRAY(uuidsToPgUUID(resourceIDs)...),
		)))

	resources := make([]*model.Resources, 0)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &resources)
	if err != nil {
		return nil, err
	}
	return lo.Map(resources, func(item *model.Resources, _ int) *entity.ResourceDetails {
		return fromDBResource(item)
	}), nil
}

func (s *Store) GetResourcesLikeName(ctx context.Context, name string) ([]*entity.ResourceDetails, error) {
	ctx = xlog.WithOperation(ctx, "store.Resources.GetResourcesLikeName")

	stmt := table.Resources.
		SELECT(table.Resources.AllColumns).
		WHERE(
			postgres.LOWER(table.Resources.Name).LIKE(
				postgres.LOWER(postgres.String("%" + name + "%"))),
		).
		LIMIT(resourceListLimit).
		ORDER_BY(table.Resources.Name.ASC())

	resources := make([]*model.Resources, 0)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &resources)
	if err != nil {
		return nil, err
	}
	return lo.Map(resources, func(r *model.Resources, _ int) *entity.ResourceDetails {
		return fromDBResource(r)
	}), nil
}
