package notifytargets

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

func (s *Service) Replace(ctx context.Context, maint *entity.Maintenance) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.Replace")
	defer span.End()

	if _, ok := dbtx.TxFromContext(ctx); ok {
		return s.replace(ctx, maint)
	}

	return s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		return s.replace(ctx, maint)
	})
}

func (s *Service) replace(ctx context.Context, maint *entity.Maintenance) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.replace")
	defer span.End()

	if err := s.notifyTargetsStore.Delete(ctx, maint.ID); err != nil {
		xlog.Error(ctx, "failed to delete notify targets", xfield.Error(err))
		return err
	}

	_, err := s.notifyTargetsStore.CreateMany(ctx, maint.ID, maint.NotifyTargets)
	if err != nil {
		xlog.Error(ctx, "failed to add notify targets", xfield.Error(err))
		return err
	}

	return nil
}
