package maint

import (
	"context"
	"errors"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) CancelMaint(ctx context.Context, cmd *entity.CancelMaintenanceCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Cancel")
	defer span.End()

	err := s.updateWithApply(ctx, cmd.MaintID, func(_ context.Context, maint *entity.Maintenance) error {
		// Already canceled: idempotent no-op (errSkipUpdate rolls the tx back
		// without a write or a duplicate cancellation notification).
		if maint.Status == entity.MaintenanceStatusCancelled {
			return errSkipUpdate
		}

		if !entity.CanMaintTransition(maint.Status, entity.MaintenanceStatusCancelled) {
			return apperr.ForbiddenMaintStatusTransition(maint.Status, entity.MaintenanceStatusCancelled)
		}

		if maint.ActualPeriod != nil {
			maint.ActualPeriod.End = lo.ToPtr(xtime.UTCNow())
		}
		maint.Status = entity.MaintenanceStatusCancelled
		maint.CancelReason = cmd.Reason
		maint.CancelReasonComment = cmd.ReasonComment
		return nil
	})
	if errors.Is(err, errSkipUpdate) {
		return nil
	}
	if err != nil {
		xlog.Error(ctx, "failed to cancel maint", xfield.Error(err))
		return err
	}

	return nil
}
