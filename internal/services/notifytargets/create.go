package notifytargets

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) Create(ctx context.Context, maintID uuid.UUID, input []*entity.NotifyTargetInput) ([]*entity.NotifyTarget, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.Create")
	defer span.End()

	notifyTargets, err := s.ResolveNotifyTarget(ctx, maintID, input)
	if err != nil {
		xlog.Error(ctx, "failed to resolve notify targets", xfield.Error(err))
		return nil, err
	}

	targets, err := s.notifyTargetsStore.CreateMany(ctx, maintID, notifyTargets)
	if err != nil {
		xlog.Error(ctx, "failed to create notify targets", xfield.Error(err))
		return nil, fmt.Errorf("failed to create notify targets: %w", err)
	}

	return targets, nil
}
