package auditor

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/eventbus"
	"github.com/ruko1202/maintmode/internal/eventbus/events"
)

// Handle конвертирует событие в entity.AuditEntry и пишет его в стор.
// Ошибка возвращается диспетчеру, который её логирует best-effort.
func (a *Listener) Handle(ctx context.Context, event eventbus.Event) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auditor.Handle")
	defer span.End()

	var entry *entity.AuditEntry
	switch e := event.(type) {
	case events.UserLoginSuccess:
		entry = &entity.AuditEntry{
			Action:           entity.AuditActionLoginSuccess,
			Actor:            e.User.Email,
			ActorID:          e.User.ID.String(),
			ActorDisplayName: e.User.Name,
			EntityType:       entity.AuditEntityTypeUser,
			EntityID:         e.User.ID.String(),
			Details:          fmt.Sprintf("login success for %s", e.User.Email),
			Metadata:         sanitizeMetadata(e.Meta),
		}
	case events.UserLoginFailed:
		entry = &entity.AuditEntry{
			Action:           entity.AuditActionLoginFailed,
			Actor:            e.User.Email,
			ActorID:          e.User.ID.String(),
			ActorDisplayName: e.User.Name,
			EntityType:       entity.AuditEntityTypeUser,
			EntityID:         e.User.ID.String(),
			Details:          fmt.Sprintf("login failed for %s", e.User.Email),
			Metadata:         sanitizeMetadata(e.Meta),
		}
	case events.UserLogoutSuccess:
		entry = &entity.AuditEntry{
			Action:           entity.AuditActionLogoutSuccess,
			Actor:            e.User.Email,
			ActorID:          e.User.ID.String(),
			ActorDisplayName: e.User.Name,
			EntityType:       entity.AuditEntityTypeUser,
			EntityID:         e.User.ID.String(),
			Details:          fmt.Sprintf("logout success for %s", e.User.Email),
			Metadata: &entity.AuditMetadata{
				SessionID:  e.SessionID,
				LogoutKind: entity.AuditLogoutKindManual,
			},
		}
	case events.UserRolesChanged:
		entry = &entity.AuditEntry{
			Action:           entity.AuditActionRolesChanged,
			Actor:            e.Actor.Email,
			ActorID:          e.Actor.ID.String(),
			ActorDisplayName: e.Actor.Name,
			EntityType:       entity.AuditEntityTypeUser,
			EntityID:         e.Target.ID.String(),
			Details:          fmt.Sprintf("roles '%s' %s for %s", e.Change.Roles, e.Kind, e.Target.Email),
			Metadata: &entity.AuditMetadata{
				Roles:             roleNames(e.Change.Roles),
				RolesAdded:        roleNames(e.Change.Added),
				RolesRemoved:      roleNames(e.Change.Removed),
				TargetEmail:       e.Target.Email,
				TargetDisplayName: e.Target.Name,
			},
		}
	case events.UserBlocked:
		entry = &entity.AuditEntry{
			Action:           entity.AuditActionUserBlocked,
			Actor:            e.Actor.Email,
			ActorID:          e.Actor.ID.String(),
			ActorDisplayName: e.Actor.Name,
			EntityType:       entity.AuditEntityTypeUser,
			EntityID:         e.Target.ID.String(),
			Details:          fmt.Sprintf("user %s blocked by %s", e.Target.Email, e.Actor.Email),
			Metadata: &entity.AuditMetadata{
				TargetEmail:       e.Target.Email,
				TargetDisplayName: e.Target.Name,
			},
		}
	case events.UserUnblocked:
		entry = &entity.AuditEntry{
			Action:           entity.AuditActionUserUnblocked,
			Actor:            e.Actor.Email,
			ActorID:          e.Actor.ID.String(),
			ActorDisplayName: e.Actor.Name,
			EntityType:       entity.AuditEntityTypeUser,
			EntityID:         e.Target.ID.String(),
			Details:          fmt.Sprintf("user %s unblocked by %s", e.Target.Email, e.Actor.Email),
			Metadata: &entity.AuditMetadata{
				TargetEmail:       e.Target.Email,
				TargetDisplayName: e.Target.Name,
			},
		}
	default:
		return fmt.Errorf("%w: %T", apperr.ErrUnsupportedEvent, e)
	}

	return a.auditorSrv.AddLog(ctx, entry)
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

func roleNames(roles []entity.Role) []string {
	return lo.Map(roles, func(r entity.Role, _ int) string { return string(r) })
}
