package calendar

import (
	"context"
	"fmt"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
)

const limitMonth = 3 * 30 * 24 * time.Hour // 90 days

func (s *Service) GetMaints(ctx context.Context, filter *calendardto.GetMaintsFilter) ([]*calendardto.Maintenance, *calendardto.MaintenancesMeta, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Calendar.GetMaints")
	defer span.End()

	if filter.PeriodTo.Sub(filter.PeriodFrom) > limitMonth {
		err := fmt.Errorf("%w: should be less or equal than 90 days", apperr.ErrInvalidPeriodInterval)
		xlog.Error(ctx, "invalid period interval", xfield.Error(err))
		return nil, nil, err
	}

	maints, truncated, err := s.maintStore.GetMaints(ctx, filter, 1000)
	if err != nil {
		xlog.Error(ctx, "failed to get maints", xfield.Error(err))
		return nil, nil, err
	}

	return lo.Map(maints, func(item *entity.Maintenance, _ int) *calendardto.Maintenance {
			return &calendardto.Maintenance{
				ID:                  item.ID,
				Title:               item.Title,
				Description:         item.Description,
				PlannedPeriod:       item.PlannedPeriod,
				ActualPeriod:        item.ActualPeriod,
				Resources:           []*calendardto.MaintenanceResource{},
				Scope:               item.Scope,
				Impact:              item.Impact,
				Status:              item.Status,
				CancelReason:        item.CancelReason,
				CancelReasonComment: item.CancelReasonComment,
				CreatedAt:           item.CreatedAt,
				UpdatedAt:           item.UpdatedAt,
				CreatedByUserID:     item.CreatedByUserID,
				ApproverUserID:      item.ApproverUserID,
			}
		}), &calendardto.MaintenancesMeta{
			Count:     int64(len(maints)),
			Truncated: truncated,
		}, nil
}
