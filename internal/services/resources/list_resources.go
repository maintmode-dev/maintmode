package resources

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ListResult is a single page of resources plus the total count of resources
// matching the same filter (used by the API to expose pagination metadata).
type ListResult struct {
	Resources []*entity.ResourceDetails
	Total     int64
}

func (s *Service) ListResources(ctx context.Context, cmd *entity.ListResourcesCmd) (*ListResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.ListResources")
	defer span.End()

	resources, total, err := s.store.List(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "failed to list resources", xfield.Error(err))
		return nil, err
	}

	return &ListResult{
		Resources: resources,
		Total:     total,
	}, nil
}
