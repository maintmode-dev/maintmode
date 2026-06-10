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

// LogChangeRoles записывает изменение ролей пользователя target актором actor.
// change.Roles — роли, затронутые действием (для replaced — итоговый набор);
// change.Added/Removed заполняются только для replaced.
func (a *Auditor) LogChangeRoles(ctx context.Context, event entity.AuditActionEvent, actor, target *entity.User, change entity.AuditRolesChange) {
	_, span := xlog.WithOperationSpan(ctx, "service.Auditor.LogChangeRoles", xfield.Any("event", event))
	defer span.End()

	a.log(ctx, &entity.AuditEntry{
		ID:               xuuid.New(),
		Action:           event.Action,
		Actor:            actor.Email,
		ActorID:          actor.ID.String(),
		ActorDisplayName: actor.Name,
		EntityType:       event.EntityType,
		EntityID:         target.ID.String(),
		Details:          fmt.Sprintf("roles '%s' %s to %s", change.Roles, event.Action, target.Email),
		Metadata: &entity.AuditMetadata{
			Roles:             roleNames(change.Roles),
			RolesAdded:        roleNames(change.Added),
			RolesRemoved:      roleNames(change.Removed),
			TargetEmail:       target.Email,
			TargetDisplayName: target.Name,
		},
		CreatedAt: xtime.UTCNow(),
	})
}
