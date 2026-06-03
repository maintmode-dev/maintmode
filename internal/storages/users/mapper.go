package users

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func fromDBUser(r *model.Users) *entity.User {
	return &entity.User{
		ID:        r.ID,
		Email:     r.Email,
		Name:      r.Name,
		Roles:     lo.Map(r.Roles, func(item string, _ int) entity.Role { return entity.Role(item) }),
		CreatedAt: r.CreatedAt,
		BlockedAt: r.BlockedAt,
	}
}

func toDBUser(r *entity.User) *model.Users {
	return &model.Users{
		Email:     r.Email,
		Name:      r.Name,
		Roles:     lo.Map(r.Roles, func(item entity.Role, _ int) string { return string(item) }),
		BlockedAt: r.BlockedAt,
	}
}
