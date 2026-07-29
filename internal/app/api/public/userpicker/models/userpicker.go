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
	// Whether the user has any messenger handle configured — an aggregate over
	// telegram and slack, never a per-transport breakdown and never the handle
	// values. Always false for callers not permitted to plan maintenances.
	//
	// Deliberately a plain bool without omitempty, so the key is present for
	// every caller: the struct is flat, and a *bool is indistinguishable from a
	// bool in the generated swagger, so key-absence could not express the gated
	// case anyway — the frontend could not tell "not permitted" from "no tag".
	// The gate lives in ToAPIAssignableUsers.
	HasMessengerTag bool `json:"has_messenger_tag"`
}

type ListAssignableUsersResponse struct {
	Users  []*AssignableUser `json:"users"`
	Total  int64             `json:"total" example:"123"`
	Limit  int64             `json:"limit" example:"50"`
	Offset int64             `json:"offset" example:"0"`
}

// ToAPIAssignableUsers projects users onto the picker shape.
//
// HasMessengerTag is an aggregate over the transports, never the handles: the
// form needs to badge people who cannot be pinged, and the values themselves
// have no use there. Who may see even that much is decided by the route's authz
// scenario, not here — the endpoint requires maintenance.create.
func ToAPIAssignableUsers(users []*entity.User) []*AssignableUser {
	return lo.Map(users, func(u *entity.User, _ int) *AssignableUser {
		return &AssignableUser{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.Name,
			Roles: lo.Map(u.Roles, func(r entity.Role, _ int) string {
				return string(r)
			}),
			HasMessengerTag: u.TelegramTag != nil || u.SlackTag != nil,
		}
	})
}
