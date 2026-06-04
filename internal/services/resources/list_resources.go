package resources

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) ListResources(ctx context.Context, cmd *entity.ListResourcesCmd) (*entity.ListResourcesResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.ListResources")
	defer span.End()

	resources, total, err := s.store.List(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "failed to list resources", xfield.Error(err))
		return nil, err
	}

	return &entity.ListResourcesResult{
		Resources: resources,
		Total:     total,
	}, nil
}
