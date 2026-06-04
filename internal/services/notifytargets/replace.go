package notifytargets

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) Replace(ctx context.Context, maint *entity.Maintenance) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.Replace")
	defer span.End()

	// WithinTx is reentrant: when called inside an existing transaction (e.g.
	// the maint update flow) it joins it, otherwise it opens its own. Either
	// way the delete+create pair is atomic.
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
