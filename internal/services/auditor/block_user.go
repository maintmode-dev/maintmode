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

func (a *Auditor) LogBlockUser(ctx context.Context, event entity.AuditActionEvent, actor, target *entity.User) {
	_, span := xlog.WithOperationSpan(ctx, "service.Auditor.LogBlockUser", xfield.Any("event", event))
	defer span.End()

	a.log(ctx, &entity.AuditEntry{
		ID:               xuuid.New(),
		Action:           event.Action,
		Actor:            actor.Email,
		ActorID:          actor.ID.String(),
		ActorDisplayName: actor.Name,
		EntityType:       event.EntityType,
		EntityID:         target.ID.String(),
		Details:          fmt.Sprintf("user %s %s by %s", target.Email, event.Action, actor.Email),
		Metadata: &entity.AuditMetadata{
			TargetEmail:       target.Email,
			TargetDisplayName: target.Name,
		},
		CreatedAt: xtime.UTCNow(),
	})
}
