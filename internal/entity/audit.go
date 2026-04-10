package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuditAction описывает тип события для аудит-лога.
type AuditAction string

const (
	AuditActionLoginSuccess  AuditAction = "success"
	AuditActionLoginFailed   AuditAction = "failed"
	AuditActionLogoutSuccess AuditAction = "success"

	AuditActionRoleAssigned  AuditAction = "assigned"
	AuditActionRoleRevoked   AuditAction = "revoked"
	AuditActionRolesReplaced AuditAction = "replaced"
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
	ID         uuid.UUID
	Action     AuditAction
	Actor      string          // кто совершил действие
	EntityType AuditEntityType // тип основной сущности: user, article, role, policy
	EntityID   string          // ID основной сущности (string, не FK)
	TargetType AuditEntityType // тип второй сущности (опционально)
	TargetID   string          // ID второй сущности (опционально)
	Details    string          // человекочитаемое описание
	CreatedAt  time.Time
}

type AuditFilter struct{}
