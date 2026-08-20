package apimodels

type Role string

const (
	RoleGuest    Role = "guest"
	RoleEditor   Role = "editor"
	RoleReviewer Role = "reviewer"
	RoleAdmin    Role = "admin"
)

var AllRoles = []Role{RoleGuest, RoleEditor, RoleReviewer, RoleAdmin}

// AssignRoleRequest is the request body for assigning a role.
type AssignRoleRequest struct {
	UserID string `json:"user_id"`
	Role   Role   `json:"role"`
}

// RevokeRoleRequest is the request body for revoking a role.
type RevokeRoleRequest struct {
	UserID string `json:"user_id"`
	Role   Role   `json:"role"`
}

type ListRolesResponse struct {
	Roles []Role `json:"roles"`
}
