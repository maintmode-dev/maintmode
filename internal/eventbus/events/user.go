package events

import "github.com/ruko1202/maintmode/internal/entity"

type UserLoginSuccess struct {
	User *entity.User
	Meta *entity.AuditMetadata
}

func (UserLoginSuccess) EventType() EventType { return EventTypeUserLoginSuccess }

type UserLoginFailed struct {
	User *entity.User
	Meta *entity.AuditMetadata
}

func (UserLoginFailed) EventType() EventType { return EventTypeUserLoginFailed }

type UserLogoutSuccess struct {
	User      *entity.User
	SessionID string
}

func (UserLogoutSuccess) EventType() EventType { return EventTypeUserLogoutSuccess }

// RolesChangeKind различает под-вид изменения ролей. Это классификация события,
// а не доменная сущность — живёт рядом с событием; аудит-листенер маппит её в
// entity.AuditAction.
type RolesChangeKind string

const (
	RolesAssigned RolesChangeKind = "assigned"
	RolesRevoked  RolesChangeKind = "revoked"
	RolesReplaced RolesChangeKind = "replaced"
)

// AuditRolesChange описывает изменение ролей для аудита.
type AuditRolesChange struct {
	Roles   []entity.Role // роли, затронутые действием (assigned/revoked) или итоговый набор (replaced)
	Added   []entity.Role // только для replaced
	Removed []entity.Role // только для replaced
}

// UserRolesChanged — изменение ролей пользователя. Kind различает
// assigned / revoked / replaced; Change несёт затронутые роли.
type UserRolesChanged struct {
	Actor  *entity.User
	Target *entity.User
	Kind   RolesChangeKind
	Change AuditRolesChange
}

func (UserRolesChanged) EventType() EventType { return EventTypeUserRolesChanged }

type UserBlocked struct {
	Actor  *entity.User
	Target *entity.User
}

func (UserBlocked) EventType() EventType { return EventTypeUserBlocked }

type UserUnblocked struct {
	Actor  *entity.User
	Target *entity.User
}

func (UserUnblocked) EventType() EventType { return EventTypeUserUnblocked }
