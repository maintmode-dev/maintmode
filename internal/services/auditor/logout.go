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

func (a *Auditor) LogLogout(ctx context.Context, event entity.AuditActionEvent, user *entity.User) {
	_, span := xlog.WithOperationSpan(ctx, "service.Auditor.Login", xfield.Any("event", event))
	defer span.End()

	a.log(ctx, &entity.AuditEntry{
		ID:         xuuid.New(),
		Action:     event.Action,
		Actor:      user.Email,
		EntityType: event.EntityType,
		EntityID:   user.ID.String(),
		TargetType: event.TargetType,
		Details:    fmt.Sprintf("logout %s for %s", event.Action, user.Email),
		CreatedAt:  xtime.UTCNow(),
	})
}
