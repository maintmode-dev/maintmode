package maint

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) Get(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Get")
	defer span.End()

	maint, err := s.maintStore.Get(ctx, maintID)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrMaintNotFound
		}
		return nil, err
	}

	resources, err := s.maintStore.GetMaintResources(ctx, []uuid.UUID{maint.ID})
	if err != nil {
		return nil, err
	}

	maint.Resources = lo.ValueOr(resources, maint.ID, []*entity.Resource{})

	return maint, nil
}
