package apiauthmodels

import (
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

type AuditLog struct {
	ID     uuid.UUID          `json:"id"`
	Action entity.AuditAction `json:"action"`
	// Actor is the actor's email (for system events, the system email).
	Actor string `json:"actor"`
	// ActorID is the stable actor ID (user UUID); empty for the system actor and
	// for unidentified users (login_failed on an unknown email).
	ActorID string `json:"actor_id,omitempty"`
	// ActorDisplayName is the actor name at event time (a snapshot, not resolved
	// again on read). Empty on records created before the field was introduced.
	ActorDisplayName string                 `json:"actor_display_name,omitempty"`
	EntityType       entity.AuditEntityType `json:"entity_type,omitempty"`
	EntityID         string                 `json:"entity_id,omitempty"`
	// Details is a human-readable description of the event (legacy/fallback string).
	Details string `json:"details,omitempty"`
	// Metadata is the structured action-specific payload for the expand grid.
	Metadata  *AuditLogMetadata `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// AuditLogMetadata carries structured, whitelist-safe details of an event.
// Which fields are populated depends on the action:
//   - login_success / login_failed: ip, user_agent, session_id (+failure_reason for failed);
//   - logout_success: session_id, logout_kind (auto|manual);
//   - assigned / revoked: roles, target_email, target_display_name;
//   - replaced: roles (resulting set), roles_added, roles_removed, target_email, target_display_name;
//   - blocked / unblocked: target_email, target_display_name.
type AuditLogMetadata struct {
	IP                string   `json:"ip,omitempty"`
	UserAgent         string   `json:"user_agent,omitempty"`
	SessionID         string   `json:"session_id,omitempty"`
	FailureReason     string   `json:"failure_reason,omitempty"`
	LogoutKind        string   `json:"logout_kind,omitempty" enums:"auto,manual"`
	Roles             []string `json:"roles,omitempty"`
	RolesAdded        []string `json:"roles_added,omitempty"`
	RolesRemoved      []string `json:"roles_removed,omitempty"`
	TargetEmail       string   `json:"target_email,omitempty"`
	TargetDisplayName string   `json:"target_display_name,omitempty"`

	// Maintenance action fields (RUK-182). Populated for maintenance.* /
	// maintenance_step.* actions:
	//   - maintenance.* / maintenance_step.*: maint_title;
	//   - maintenance.updated: changes (before/after per changed scalar).
	MaintTitle string                `json:"maint_title,omitempty"`
	Changes    []AuditLogFieldChange `json:"changes,omitempty"`
}

// AuditLogFieldChange is one before/after entry in a maintenance.updated diff.
type AuditLogFieldChange struct {
	Field string `json:"field"`
	Old   string `json:"old,omitempty"`
	New   string `json:"new,omitempty"`
}

// AuditFacets carries per-category entry counts computed in the current
// actor/date filter window (without the action filter).
type AuditFacets struct {
	All         int64 `json:"all" example:"123"`
	Auth        int64 `json:"auth" example:"42"`
	Roles       int64 `json:"roles" example:"8"`
	Block       int64 `json:"block" example:"1"`
	Maintenance int64 `json:"maintenance" example:"17"`
	Integration int64 `json:"integration" example:"3"`
}

type AuditLogResponse struct {
	Logs   []*AuditLog `json:"logs"`
	Total  int64       `json:"total" example:"123"`
	Facets AuditFacets `json:"facets"`
}
