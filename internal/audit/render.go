package audit

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Renderer turns an audited Action into the rendered, point-in-time payload the
// audit-write outbox task carries. The occurred-at time and the per-event id are
// stamped at render time (publish time), so ordering and idempotency are decided
// once, regardless of when the processor drains the task.
type Renderer struct {
	now   func() time.Time
	newID func() uuid.UUID
}

// NewRenderer builds a Renderer with the production clock and id source.
func NewRenderer() *Renderer {
	return &Renderer{
		now:   xtime.UTCNow,
		newID: uuid.New,
	}
}

// Render builds the snapshot payload for action.
func (r *Renderer) Render(action Action) (entity.ProcessorTaskPayloadAuditWrite, error) {
	payload := entity.ProcessorTaskPayloadAuditWrite{
		EventID:    r.newID(),
		OccurredAt: r.now(),
		EntityType: entity.AuditEntityTypeUser,
		Action:     action.auditAction(),
	}

	if err := fillPayload(&payload, action); err != nil {
		return entity.ProcessorTaskPayloadAuditWrite{}, err
	}

	return payload, nil
}

// fillPayload maps the action-specific fields onto payload. EntityType, EventID,
// OccurredAt and Action are already set by the caller.
func fillPayload(payload *entity.ProcessorTaskPayloadAuditWrite, action Action) error {
	switch a := action.(type) {
	case LoginSuccess:
		setActor(payload, a.User)
		payload.EntityID = a.User.ID.String()
		payload.Details = fmt.Sprintf("login success for %s", a.User.Email)
		payload.Metadata = sanitizeMetadata(a.Meta)
	case LoginFailed:
		setActor(payload, a.User)
		payload.EntityID = a.User.ID.String()
		payload.Details = fmt.Sprintf("login failed for %s", a.User.Email)
		payload.Metadata = sanitizeMetadata(a.Meta)
	case LogoutSuccess:
		setActor(payload, a.User)
		payload.EntityID = a.User.ID.String()
		payload.Details = fmt.Sprintf("logout success for %s", a.User.Email)
		payload.Metadata = &entity.AuditMetadata{
			SessionID:  a.SessionID,
			LogoutKind: entity.AuditLogoutKindManual,
		}
	case RolesChanged:
		setActor(payload, a.Actor)
		payload.EntityID = a.Target.ID.String()
		payload.Details = fmt.Sprintf("roles '%s' %s for %s", a.Change.Roles, a.Kind, a.Target.Email)
		payload.Metadata = &entity.AuditMetadata{
			Roles:             roleNames(a.Change.Roles),
			RolesAdded:        roleNames(a.Change.Added),
			RolesRemoved:      roleNames(a.Change.Removed),
			TargetEmail:       a.Target.Email,
			TargetDisplayName: a.Target.Name,
		}
	case UserBlocked:
		setActor(payload, a.Actor)
		payload.EntityID = a.Target.ID.String()
		payload.Details = fmt.Sprintf("user %s blocked by %s", a.Target.Email, a.Actor.Email)
		payload.Metadata = &entity.AuditMetadata{
			TargetEmail:       a.Target.Email,
			TargetDisplayName: a.Target.Name,
		}
	case UserUnblocked:
		setActor(payload, a.Actor)
		payload.EntityID = a.Target.ID.String()
		payload.Details = fmt.Sprintf("user %s unblocked by %s", a.Target.Email, a.Actor.Email)
		payload.Metadata = &entity.AuditMetadata{
			TargetEmail:       a.Target.Email,
			TargetDisplayName: a.Target.Name,
		}
	default:
		return fmt.Errorf("%w: %T", apperr.ErrUnsupportedEvent, a)
	}

	return nil
}

// setActor fills the actor identity fields from the user who performed the
// action. actor_display_name is the point-in-time snapshot (not resolved on
// read). Note: for an unidentified actor (e.g. a login that failed before the
// user was resolved) ID is the zero UUID and ActorID becomes its all-zeros
// string — preserved verbatim from the pre-RUK-179 listener behavior.
func setActor(payload *entity.ProcessorTaskPayloadAuditWrite, actor *entity.User) {
	payload.Actor = actor.Email
	payload.ActorID = actor.ID.String()
	payload.ActorDisplayName = actor.Name
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
