package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) CreateResource(ctx context.Context, cmd *entity.CreateResourceCmd) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.CreateResource")
	defer span.End()

	var resource *entity.ResourceDetails
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		resource, err = s.store.GetByName(ctx, cmd.Name)

		switch {
		case errors.Is(err, apperr.ErrResourceNotFound):
			resource, err = s.store.Create(ctx, &entity.ResourceDetails{
				Name:        cmd.Name,
				Description: cmd.Description,
				ExternalID:  cmd.ExternalID,
			})
			if err != nil {
				return fmt.Errorf("failed to create resource: %w", err)
			}
		case err != nil:
			return fmt.Errorf("failed to check existing resource: %w", err)
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to create resource", xfield.String("name", cmd.Name), xfield.Error(err))
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	return resource, nil
}
