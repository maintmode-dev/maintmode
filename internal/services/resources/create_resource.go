package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func (s *Service) CreateResource(ctx context.Context, cmd *entity.CreateResourceCmd) (*entity.ResourceDetails, error) {
	ctx = xlog.WithOperation(ctx, "service.Resources.CreateResource")

	resource := &entity.ResourceDetails{
		ID:          xuuid.New(),
		Name:        cmd.Name,
		Description: cmd.Description,
		ExternalID:  cmd.ExternalID,
		CreatedAt:   xtime.UTCNow(),
	}

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		existing, err := s.store.GetByName(ctx, resource.Name)
		if err != nil && !errors.Is(err, apperr.ErrResourceNotFound) {
			return fmt.Errorf("failed to check existing resource: %w", err)
		}

		if existing != nil {
			return apperr.ErrResourceAlreadyExists
		}

		err = s.store.Create(ctx, resource)
		if err != nil {
			return fmt.Errorf("failed to create resource: %w", err)
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to create resource", xfield.String("name", resource.Name), xfield.Error(err))
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	return resource, nil
}
