package auditor

import (
	"context"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/eventbus"
	"github.com/ruko1202/maintmode/internal/eventbus/events"
)

// auditWriter is the write-side of the audit service the listener depends on.
// Defined consumer-side (not the concrete *auditor.Auditor) so Handle can be
// unit-tested with a fake and the listener stays decoupled from storage.
type auditWriter interface {
	AddLog(ctx context.Context, entry *entity.AuditEntry) error
}

// Listener реализует eventbus.Listener: пишет аудит-лог по write-событиям
// (login / logout / roles / block). Read-сторона (GetLogs) живёт отдельно.
var _ eventbus.Listener = (*Listener)(nil)

type Listener struct {
	auditorSrv auditWriter
}

func NewListener(auditorSrv auditWriter) *Listener {
	return &Listener{
		auditorSrv: auditorSrv,
	}
}

// SupportEvents list available events for this listener.
func (a *Listener) SupportEvents() []events.EventType {
	return []events.EventType{
		events.EventTypeUserLoginSuccess,
		events.EventTypeUserLoginFailed,
		events.EventTypeUserLogoutSuccess,

		events.EventTypeUserRolesChanged,

		events.EventTypeUserBlocked,
		events.EventTypeUserUnblocked,
	}
}
