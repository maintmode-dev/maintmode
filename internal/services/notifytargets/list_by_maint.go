package notifytargets

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.NotifyTarget, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.ListByMaint")
	defer span.End()

	targets, err := s.notifyTargetsStore.ListByMaint(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "failed to list notify targets", xfield.Error(err))
		return nil, fmt.Errorf("failed to list notify targets: %w", err)
	}

	return targets, nil
}
