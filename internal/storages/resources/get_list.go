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

func (s *Store) GetResourcesDetails(ctx context.Context, resourceIDs []uuid.UUID) ([]*entity.ResourceDetails, error) {
	ctx = xlog.WithOperation(ctx, "store.Resources.GetByID")

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
