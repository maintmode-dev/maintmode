package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuditAction описывает тип события для аудит-лога.
type AuditAction string

const (
	AuditActionLoginSuccess  AuditAction = "login_success"
	AuditActionLoginFailed   AuditAction = "login_failed"
	AuditActionLogoutSuccess AuditAction = "logout_success"

	AuditActionRoleAssigned  AuditAction = "assigned"
	AuditActionRoleRevoked   AuditAction = "revoked"
	AuditActionRolesReplaced AuditAction = "replaced"

	AuditActionUserBlocked   AuditAction = "blocked"
	AuditActionUserUnblocked AuditAction = "unblocked"
)

type AuditEntityType string

const (
	auditEntityTypeRole   = "role"
	auditEntityTypeLogin  = "login"
	auditEntityTypeLogout = "logout"
	auditEntityTypeUser   = "user"
)

type AuditActionEvent struct {
	TargetType AuditEntityType
	EntityType AuditEntityType
	Action     AuditAction
}

var (
	// RBAC: роли
	AuditEventRoleAssigned = AuditActionEvent{TargetType: auditEntityTypeUser, EntityType: auditEntityTypeRole, Action: AuditActionRoleAssigned}
	AuditEventRoleRevoked  = AuditActionEvent{TargetType: auditEntityTypeUser, EntityType: auditEntityTypeRole, Action: AuditActionRoleRevoked}
	AuditEventRoleReplaced = AuditActionEvent{TargetType: auditEntityTypeUser, EntityType: auditEntityTypeRole, Action: AuditActionRolesReplaced}

	// RBAC: блокировка пользователей
	AuditEventUserBlocked   = AuditActionEvent{TargetType: auditEntityTypeUser, EntityType: auditEntityTypeUser, Action: AuditActionUserBlocked}
	AuditEventUserUnblocked = AuditActionEvent{TargetType: auditEntityTypeUser, EntityType: auditEntityTypeUser, Action: AuditActionUserUnblocked}

	// Аутентификация
	AuditEventLoginSuccess  = AuditActionEvent{TargetType: auditEntityTypeUser, EntityType: auditEntityTypeLogin, Action: AuditActionLoginSuccess}
	AuditEventLoginFailed   = AuditActionEvent{TargetType: auditEntityTypeUser, EntityType: auditEntityTypeLogin, Action: AuditActionLoginFailed}
	AuditEventLogoutSuccess = AuditActionEvent{TargetType: auditEntityTypeUser, EntityType: auditEntityTypeLogout, Action: AuditActionLogoutSuccess}
)

// AuditEntry представляет структурированную запись аудит-лога.
//
// Дизайн:
//   - EntityType + EntityID — привязка к конкретной сущности для быстрого поиска.
//     НЕ foreign key — сущность может быть удалена, аудит остаётся.
//   - TargetType + TargetID — вторая сущность для действий вида
//     "actor сделал action над entity, затрагивая target".
//     Пример: admin(actor) assigned(action) role(entity=editor) to user(target=alice).
//   - EntityID хранится как string (не UUID) — чтобы не ломаться при удалении сущности
//     и поддерживать разные типы ID (UUID, string name, int).
type AuditEntry struct {
	ID               uuid.UUID
	Action           AuditAction
	Actor            string          // кто совершил действие (email)
	ActorID          string          // стабильный ID актора (user UUID, string — не FK); пустой для system
	ActorDisplayName string          // снапшот имени актора на момент события (не резолвится на чтении)
	EntityType       AuditEntityType // тип основной сущности: user, article, role, policy
	EntityID         string          // ID основной сущности (string, не FK)
	TargetType       AuditEntityType // тип второй сущности (опционально)
	TargetID         string          // ID второй сущности (опционально)
	Details          string          // человекочитаемое описание
	Metadata         *AuditMetadata  // структурированный action-specific payload (опционально)
	CreatedAt        time.Time
}

// AuditMetadata — структурированный, action-specific payload записи аудита.
// Строго whitelist безопасных полей: IP, user agent, session id, имена ролей.
// НИКОГДА не класть сюда токены, куки, секреты или сырые payload'ы (см. RUK-81).
//
// Заполненность зависит от action:
//   - login_success / login_failed: IP, UserAgent, SessionID (+FailureReason для failed);
//   - logout_success: SessionID, LogoutKind;
//   - assigned / revoked: Roles, TargetEmail, TargetDisplayName;
//   - replaced: Roles (итоговый набор), RolesAdded, RolesRemoved, TargetEmail, TargetDisplayName;
//   - blocked / unblocked: TargetEmail, TargetDisplayName.
type AuditMetadata struct {
	IP                string   `json:"ip,omitempty"`
	UserAgent         string   `json:"user_agent,omitempty"`
	SessionID         string   `json:"session_id,omitempty"`
	FailureReason     string   `json:"failure_reason,omitempty"`
	LogoutKind        string   `json:"logout_kind,omitempty"` // auto | manual
	Roles             []string `json:"roles,omitempty"`
	RolesAdded        []string `json:"roles_added,omitempty"`
	RolesRemoved      []string `json:"roles_removed,omitempty"`
	TargetEmail       string   `json:"target_email,omitempty"`
	TargetDisplayName string   `json:"target_display_name,omitempty"`
}

const (
	AuditLogoutKindManual = "manual"
	AuditLogoutKindAuto   = "auto"
)

// AuditRolesChange описывает изменение ролей для аудита.
type AuditRolesChange struct {
	Roles   []Role // роли, затронутые действием (assigned/revoked) или итоговый набор (replaced)
	Added   []Role // только для replaced
	Removed []Role // только для replaced
}

// AuditFilter is a read-time filter for audit log entries.
// All fields are optional; a nil pointer means "do not filter by this field".
type AuditFilter struct {
	Action      *AuditAction
	Actor       *string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}
