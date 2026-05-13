package auditor

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (a *Auditor) GetLogs(ctx context.Context, cmd *entity.GetAuditLogsCmd) ([]*entity.AuditEntry, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auditor.GetLogs")
	defer span.End()

	if cmd.Filter == nil {
		cmd.Filter = &entity.AuditFilter{}
	}

	if cmd.Filter.CreatedFrom != nil && cmd.Filter.CreatedTo != nil {
		from, to := lo.FromPtr(cmd.Filter.CreatedFrom), lo.FromPtr(cmd.Filter.CreatedTo)
		if from.After(to) {
			return nil, fmt.Errorf("%w: created_from must be <= created_to", apperr.ErrValidation)
		}
	}

	return a.store.GetLogs(ctx, cmd)
}
