package maint

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetMaint(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Get")
	defer span.End()

	maint, err := s.maintStore.GetMaint(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "failed to get maint", xfield.Error(err))
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrMaintNotFound
		}
		return nil, err
	}

	resources, err := s.maintStore.GetMaintResources(ctx, []uuid.UUID{maint.ID})
	if err != nil {
		xlog.Error(ctx, "failed to get maint resources", xfield.Error(err))
		return nil, err
	}

	maint.Resources = lo.Map(resources[maint.ID], func(r *entity.ResourceDetails, _ int) uuid.UUID {
		return r.ID
	})

	steps, err := s.maintStore.GetMaintSteps(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "failed to get maint steps", xfield.Error(err))
		return nil, err
	}
	maint.Steps = steps

	targets, err := s.notifyTargets.ListByMaint(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "failed to get notify targets", xfield.Error(err))
		return nil, err
	}
	maint.NotifyTargets = targets

	deferred, err := s.deferred.ListByMaint(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "failed to get deferred notifications", xfield.Error(err))
		return nil, err
	}
	maint.DeferredNotifications = deferred

	mentions, err := s.maintStore.GetMaintMentions(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "failed to get maint mentions", xfield.Error(err))
		return nil, err
	}
	maint.Mentions = mentions

	return maint, nil
}
