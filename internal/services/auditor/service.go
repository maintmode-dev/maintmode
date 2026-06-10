package auditor

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

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

func roleNames(roles []entity.Role) []string {
	return lo.Map(roles, func(r entity.Role, _ int) string { return string(r) })
}

// maxUserAgentLen ограничивает User-Agent в метаданных аудита: заголовок
// приходит от клиента и без лимита раздувает каждую запись лога.
const maxUserAgentLen = 256

func sanitizeMetadata(m *entity.AuditMetadata) *entity.AuditMetadata {
	if m == nil {
		return nil
	}
	if len(m.UserAgent) > maxUserAgentLen {
		runes := []rune(m.UserAgent)
		if len(runes) > maxUserAgentLen {
			m.UserAgent = string(runes[:maxUserAgentLen])
		}
	}
	return m
}
