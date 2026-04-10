package apimodels

type Role string

const (
	RoleGuest    Role = "guest"
	RoleEditor   Role = "editor"
	RoleReviewer Role = "reviewer"
	RoleAdmin    Role = "admin"
)

var AllRoles = []Role{RoleGuest, RoleEditor, RoleReviewer, RoleAdmin}

// AssignRoleRequest — тело запроса на назначение роли.
type AssignRoleRequest struct {
	UserID string `json:"user_id"`
	Role   Role   `json:"role"`
}

// RevokeRoleRequest — тело запроса на отзыв роли.
type RevokeRoleRequest struct {
	UserID string `json:"user_id"`
	Role   Role   `json:"role"`
}

type ListRolesResponse struct {
	Roles []Role `json:"roles"`
}
