package auditor

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// LogLogin записывает событие логина. meta — whitelist-safe контекст запроса
// (IP, user agent, session id, причина отказа); может быть nil.
func (a *Auditor) LogLogin(ctx context.Context, event entity.AuditActionEvent, user *entity.User, meta *entity.AuditMetadata) {
	_, span := xlog.WithOperationSpan(ctx, "service.Auditor.Login", xfield.Any("event", event))
	defer span.End()

	a.log(ctx, &entity.AuditEntry{
		ID:               xuuid.New(),
		Action:           event.Action,
		Actor:            user.Email,
		ActorID:          user.ID.String(),
		ActorDisplayName: user.Name,
		EntityType:       event.EntityType,
		EntityID:         user.ID.String(),
		Details:          fmt.Sprintf("login %s for %s", event.Action, user.Email),
		Metadata:         sanitizeMetadata(meta),
		CreatedAt:        xtime.UTCNow(),
	})
}
