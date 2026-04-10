package auditor

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (a *Auditor) GetLogs(ctx context.Context, lastN int64) ([]*entity.AuditEntry, error) {
	_, span := xlog.WithOperationSpan(ctx, "service.Auditor.GetLogs")
	defer span.End()

	return a.store.GetLogs(ctx, &entity.AuditFilter{}, lastN)
}
