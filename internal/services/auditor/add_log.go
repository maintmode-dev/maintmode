package auditor

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (a *Auditor) AddLog(ctx context.Context, entry *entity.AuditEntry) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auditor.AddLog")
	defer span.End()

	return a.store.AddLog(ctx, entry)
}
