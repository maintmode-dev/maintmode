package useridentities

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func fromDB(r *model.UserIdentities) *entity.UserIdentity {
	return &entity.UserIdentity{
		ID:            r.ID,
		UserID:        r.UserID,
		IntegrationID: &r.IntegrationID,
		Subject:       r.Subject,
		Email:         r.Email,
		CreatedAt:     r.CreatedAt,
	}
}

func toDB(r *entity.UserIdentity) *model.UserIdentities {
	return &model.UserIdentities{
		UserID:        r.UserID,
		IntegrationID: lo.FromPtr(r.IntegrationID),
		Subject:       r.Subject,
		Email:         r.Email,
	}
}
