package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuditAction описывает тип события для аудит-лога.
type AuditAction string

const (
	AuditActionLoginSuccess  AuditAction = "login.success"
	AuditActionLoginFailed   AuditAction = "login.failed"
	AuditActionLogoutSuccess AuditAction = "logout.success"

	AuditActionRolesChanged AuditAction = "roles.changed"

	AuditActionUserBlocked   AuditAction = "user.blocked"
	AuditActionUserUnblocked AuditAction = "user.unblocked"

	// Maintenance lifecycle/CRUD actions (RUK-182). AuditActionMaintAutoCanceled
	// is the automatic overdue-cancel path (RUK-181) — system actor, no human.
	// Values use the project's "canceled" spelling (matches MaintenanceStatusCancelled = "canceled").
	AuditActionMaintCreated   AuditAction = "maintenance.created"
	AuditActionMaintUpdated   AuditAction = "maintenance.updated"
	AuditActionMaintApproved  AuditAction = "maintenance.approved"
	AuditActionMaintStarted   AuditAction = "maintenance.started"
	AuditActionMaintCompleted AuditAction = "maintenance.completed"
	AuditActionMaintCanceled  AuditAction = "maintenance.canceled"

	AuditActionMaintStepStarted   AuditAction = "maintenance_step.started"
	AuditActionMaintStepCompleted AuditAction = "maintenance_step.completed"
	AuditActionMaintStepCanceled  AuditAction = "maintenance_step.canceled"
)

func (a AuditAction) IsValid() bool {
	switch a {
	case AuditActionLoginSuccess,
		AuditActionLoginFailed,
		AuditActionLogoutSuccess,
		AuditActionRolesChanged,
		AuditActionUserBlocked,
		AuditActionUserUnblocked,
		AuditActionMaintCreated,
		AuditActionMaintUpdated,
		AuditActionMaintApproved,
		AuditActionMaintStarted,
		AuditActionMaintCompleted,
		AuditActionMaintCanceled,
		AuditActionMaintStepStarted,
		AuditActionMaintStepCompleted,
		AuditActionMaintStepCanceled:
		return true
	default:
		return false
	}
}

type AuditEntityType string

const (
	AuditEntityTypeUser        AuditEntityType = "user"
	AuditEntityTypeMaintenance AuditEntityType = "maintenance"
)

// AuditEntry представляет структурированную запись аудит-лога.
//
// Дизайн:
//   - EntityType + EntityID — привязка к конкретной сущности для быстрого поиска.
//     НЕ foreign key — сущность может быть удалена, аудит остаётся.
//   - EntityID хранится как string (не UUID) — чтобы не ломаться при удалении сущности
//     и поддерживать разные типы ID (UUID, string name, int).
type AuditEntry struct {
	ID               uuid.UUID
	EventID          uuid.UUID // per-event idempotency key (RUK-179); uuid.Nil for legacy/non-outbox writes
	Action           AuditAction
	Actor            string          // кто совершил действие (email)
	ActorID          string          // стабильный ID актора (user UUID, string — не FK); пустой для system
	ActorDisplayName string          // снапшот имени актора на момент события (не резолвится на чтении)
	EntityID         string          // ID основной сущности (string, не FK)
	EntityType       AuditEntityType // тип основной сущности: user, maint, etc
	Details          string          // человекочитаемое описание
	Metadata         *AuditMetadata  // структурированный action-specific payload (опционально)
	CreatedAt        time.Time
}

type AuditFailureReason string

// Whitelist-safe причины отказа логина для аудит-метаданных. Сырой текст
// ошибки в аудит не пишется — он может содержать внутренние детали (RUK-81).
const (
	AuditFailureUserProvisioning AuditFailureReason = "user provisioning failed"
	//nolint:gosec // G101 false positive: человекочитаемая причина отказа, не credential
	AuditFailureTokenIssuance AuditFailureReason = "token issuance failed"
)

type AuditLogoutKind string

const (
	AuditLogoutKindManual = "manual"
	AuditLogoutKindAuto   = "auto"
)

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
	IP                string             `json:"ip,omitempty"`
	UserAgent         string             `json:"user_agent,omitempty"`
	SessionID         string             `json:"session_id,omitempty"`
	FailureReason     AuditFailureReason `json:"failure_reason,omitempty"`
	LogoutKind        AuditLogoutKind    `json:"logout_kind,omitempty"`
	Roles             []string           `json:"roles,omitempty"`
	RolesAdded        []string           `json:"roles_added,omitempty"`
	RolesRemoved      []string           `json:"roles_removed,omitempty"`
	TargetEmail       string             `json:"target_email,omitempty"`
	TargetDisplayName string             `json:"target_display_name,omitempty"`

	// Maintenance action fields (RUK-182). All omitempty; populated only for
	// maintenance.* / maintenance_step.* actions:
	//   - maintenance.* / maintenance_step.*: MaintTitle;
	//   - maintenance.updated: Changes (before/after per changed scalar).
	// The cancel reason is not duplicated here — it lives on the maintenance's own
	// cancel_reason column; the audit row carries it only in the Details string.
	MaintTitle string             `json:"maint_title,omitempty"`
	Changes    []AuditFieldChange `json:"changes,omitempty"`
}

// AuditFieldChange is one before/after entry in a maintenance.updated diff.
// Old/New are rendered string snapshots of a scalar field (title, planned
// window, scope, impact, approver). Collection fields (steps, targets) record a
// changed flag via Field with empty Old/New rather than noisy element diffs.
type AuditFieldChange struct {
	Field string `json:"field"`
	Old   string `json:"old,omitempty"`
	New   string `json:"new,omitempty"`
}

// AuditCategory groups audit actions into the FE filter chips
// (Auth / Roles / Block / Maintenance). The category -> actions mapping is owned
// by the backend so facet counts and category expansion stay consistent.
type AuditCategory string

const (
	AuditCategoryAuth        AuditCategory = "auth"
	AuditCategoryRoles       AuditCategory = "roles"
	AuditCategoryBlock       AuditCategory = "block"
	AuditCategoryMaintenance AuditCategory = "maintenance"
)

var auditActionCategories = map[AuditAction]AuditCategory{
	AuditActionLoginSuccess:  AuditCategoryAuth,
	AuditActionLoginFailed:   AuditCategoryAuth,
	AuditActionLogoutSuccess: AuditCategoryAuth,

	AuditActionRolesChanged: AuditCategoryRoles,

	AuditActionUserBlocked:   AuditCategoryBlock,
	AuditActionUserUnblocked: AuditCategoryBlock,

	// Maintenance lifecycle/CRUD + steps all share one FE category chip; steps
	// belong to a maintenance and are not split out (RUK-182).
	AuditActionMaintCreated:       AuditCategoryMaintenance,
	AuditActionMaintUpdated:       AuditCategoryMaintenance,
	AuditActionMaintApproved:      AuditCategoryMaintenance,
	AuditActionMaintStarted:       AuditCategoryMaintenance,
	AuditActionMaintCompleted:     AuditCategoryMaintenance,
	AuditActionMaintCanceled:      AuditCategoryMaintenance,
	AuditActionMaintStepStarted:   AuditCategoryMaintenance,
	AuditActionMaintStepCompleted: AuditCategoryMaintenance,
	AuditActionMaintStepCanceled:  AuditCategoryMaintenance,
}

// AuditActionCategory returns the facet category of action.
// ok is false for actions outside the known set.
func AuditActionCategory(action AuditAction) (AuditCategory, bool) {
	category, ok := auditActionCategories[action]
	return category, ok
}

// AuditFilter is a read-time filter for audit log entries.
// All fields are optional; a nil pointer / empty slice means
// "do not filter by this field".
type AuditFilter struct {
	Actions     []AuditAction
	Actor       *string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

// WithoutActions returns a copy of the filter with the action filter dropped.
// Facet counts are computed in the actor/date window regardless of the
// selected category, so the chips keep wayfinding numbers.
func (f *AuditFilter) WithoutActions() *AuditFilter {
	if f == nil {
		return nil
	}
	clone := *f
	clone.Actions = nil
	return &clone
}

// AuditFacets carries per-category entry counts within the current
// actor/date filter window. All is the count across every action.
type AuditFacets struct {
	All         int64
	Auth        int64
	Roles       int64
	Block       int64
	Maintenance int64
}

// AuditLogsPage is one page of audit log entries plus pagination/facet
// metadata computed under the same filter.
type AuditLogsPage struct {
	Logs   []*AuditEntry
	Total  int64
	Facets AuditFacets
}
