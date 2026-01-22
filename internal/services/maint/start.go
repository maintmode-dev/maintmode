package maint

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func (s *Service) Start(ctx context.Context, cmd *entity.StartMaintenanceCmd) error {
	ctx = xlog.WithOperation(ctx, "service.Maintenance.Start")

	return s.updateWithApply(ctx, cmd.MaintID, func(_ context.Context, maint *entity.Maintenance) error {
		if !entity.CanTransition(maint.Status, entity.MaintenanceStatusInProgress) {
			return apperr.ForbiddenStatusTransition(maint.Status)
		}

		maint.ActualPeriod = lo.ToPtr(entity.NewOpenEndedPeriod(xtime.UTCNow()))
		maint.Status = entity.MaintenanceStatusInProgress
		return nil
	})
}
