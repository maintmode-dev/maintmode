package maint

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func (s *Service) StartMaint(ctx context.Context, cmd *entity.StartMaintenanceCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Start")
	defer span.End()

	_, current, err := s.updateWithApply(ctx, cmd.MaintID,
		func(_ context.Context, maint *entity.Maintenance) error {
			if !entity.CanMaintTransition(maint.Status, entity.MaintenanceStatusInProgress) {
				return apperr.ForbiddenMaintStatusTransition(maint.Status, entity.MaintenanceStatusInProgress)
			}

			maint.ActualPeriod = lo.ToPtr(entity.NewOpenEndedPeriod(xtime.UTCNow()))
			maint.Status = entity.MaintenanceStatusInProgress
			return nil
		},
	)
	if err != nil {
		xlog.Error(ctx, "failed to start maint", xfield.Error(err))
		return err
	}

	s.publishAudit(ctx, audit.MaintStarted{Actor: cmd.Actor, Maint: current})
	return nil
}
