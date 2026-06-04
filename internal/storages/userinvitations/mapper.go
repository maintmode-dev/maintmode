package userinvitations

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func fromDB(r *model.UserInvitations) *entity.Invitation {
	return &entity.Invitation{
		ID:          r.ID,
		Email:       r.Email,
		Roles:       lo.Map(r.Roles, func(item string, _ int) entity.Role { return entity.Role(item) }),
		TokenHash:   r.TokenHash,
		Status:      entity.InvitationStatus(r.Status),
		InvitedByID: r.InvitedByID,
		ExpiresAt:   r.ExpiresAt,
		SentAt:      r.SentAt,
		AcceptedAt:  r.AcceptedAt,
		RevokedAt:   r.RevokedAt,
		CreatedAt:   r.CreatedAt,
	}
}

func toDB(r *entity.Invitation) *model.UserInvitations {
	return &model.UserInvitations{
		ID:          r.ID,
		Email:       r.Email,
		Roles:       lo.Map(r.Roles, func(item entity.Role, _ int) string { return string(item) }),
		TokenHash:   r.TokenHash,
		Status:      string(r.Status),
		InvitedByID: r.InvitedByID,
		ExpiresAt:   r.ExpiresAt,
		SentAt:      r.SentAt,
		AcceptedAt:  r.AcceptedAt,
		RevokedAt:   r.RevokedAt,
	}
}
