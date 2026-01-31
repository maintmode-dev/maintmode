package maint

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) Cancel(ctx context.Context, cmd *entity.CancelMaintenanceCmd) error {
	ctx = xlog.WithOperation(ctx, "service.Maint.Cancel")

	return s.updateWithApply(ctx, cmd.MaintID, func(_ context.Context, maint *entity.Maintenance) error {
		if maint.Status == entity.MaintenanceStatusCancelled {
			return nil
		}

		if !entity.CanTransition(maint.Status, entity.MaintenanceStatusCancelled) {
			return apperr.ForbiddenStatusTransition(maint.Status)
		}

		if maint.ActualPeriod != nil {
			maint.ActualPeriod.End = lo.ToPtr(xtime.UTCNow())
		}
		maint.Status = entity.MaintenanceStatusCancelled
		maint.CancelReason = cmd.Reason
		maint.CancelReasonComment = cmd.ReasonComment
		return nil
	})
}
