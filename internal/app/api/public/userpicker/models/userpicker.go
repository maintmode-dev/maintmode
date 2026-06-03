package apimodels

import (
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// AssignableUser is one selectable user in a maintenance assignment picker.
type AssignableUser struct {
	ID          uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Roles       []string  `json:"roles"`
}

type ListAssignableUsersResponse struct {
	Users  []*AssignableUser `json:"users"`
	Total  int64             `json:"total" example:"123"`
	Limit  int64             `json:"limit" example:"50"`
	Offset int64             `json:"offset" example:"0"`
}

func ToAPIAssignableUsers(users []*entity.User) []*AssignableUser {
	return lo.Map(users, func(u *entity.User, _ int) *AssignableUser {
		return &AssignableUser{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.Name,
			Roles: lo.Map(u.Roles, func(r entity.Role, _ int) string {
				return string(r)
			}),
		}
	})
}
