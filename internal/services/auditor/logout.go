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

// LogLogout записывает событие логаута. sessionID — jti access-токена,
// завершившего сессию (refresh family на этом этапе уже недоступна).
func (a *Auditor) LogLogout(ctx context.Context, event entity.AuditActionEvent, user *entity.User, sessionID string) {
	_, span := xlog.WithOperationSpan(ctx, "service.Auditor.Logout", xfield.Any("event", event))
	defer span.End()

	a.log(ctx, &entity.AuditEntry{
		ID:               xuuid.New(),
		Action:           event.Action,
		Actor:            user.Email,
		ActorID:          user.ID.String(),
		ActorDisplayName: user.Name,
		EntityType:       event.EntityType,
		EntityID:         user.ID.String(),
		Details:          fmt.Sprintf("logout %s for %s", event.Action, user.Email),
		Metadata: &entity.AuditMetadata{
			SessionID:  sessionID,
			LogoutKind: entity.AuditLogoutKindManual,
		},
		CreatedAt: xtime.UTCNow(),
	})
}
