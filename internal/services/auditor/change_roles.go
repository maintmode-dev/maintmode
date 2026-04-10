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

func (a *Auditor) LogChangeRoles(ctx context.Context, event entity.AuditActionEvent, actor, target *entity.User, roles []entity.Role) {
	_, span := xlog.WithOperationSpan(ctx, "service.Auditor.LogChangeRoles", xfield.Any("event", event))
	defer span.End()

	a.log(ctx, &entity.AuditEntry{
		ID:         xuuid.New(),
		Action:     event.Action,
		Actor:      actor.Email,
		EntityType: event.EntityType,
		EntityID:   target.ID.String(),
		TargetType: event.TargetType,
		TargetID:   target.Email,
		Details:    fmt.Sprintf("roles '%s' %s to %s", roles, event.Action, target.Email),
		CreatedAt:  xtime.UTCNow(),
	})
}
