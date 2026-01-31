package maint

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) Complete(ctx context.Context, cmd *entity.CompleteMaintenanceCmd) error {
	ctx = xlog.WithOperation(ctx, "service.Maint.Complete")

	return s.updateWithApply(ctx, cmd.MaintID, func(_ context.Context, maint *entity.Maintenance) error {
		if maint.Status == entity.MaintenanceStatusCompleted {
			return nil
		}

		if !entity.CanTransition(maint.Status, entity.MaintenanceStatusCompleted) {
			return apperr.ForbiddenStatusTransition(maint.Status)
		}

		maint.ActualPeriod.End = lo.ToPtr(xtime.UTCNow())
		maint.Status = entity.MaintenanceStatusCompleted
		return nil
	})
}
