package auditor

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/audit"
)

type Auditor struct {
	store *audit.Store
}

func NewAuditor(store *audit.Store) *Auditor {
	return &Auditor{
		store: store,
	}
}

func (a *Auditor) log(ctx context.Context, event *entity.AuditEntry) {
	err := a.store.AddLog(ctx, event)
	if err != nil {
		xlog.Error(ctx, "failed to add audit log", xfield.Error(err))
	}
}
