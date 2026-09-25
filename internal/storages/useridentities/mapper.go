package useridentities

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func fromDB(r *model.UserIdentities) *entity.UserIdentity {
	identity := &entity.UserIdentity{
		ID:            r.ID,
		UserID:        r.UserID,
		IntegrationID: r.IntegrationID,
		Subject:       r.Subject,
		Email:         r.Email,
		CreatedAt:     r.CreatedAt,
	}

	// Converted rather than copied: the column is a plain string and the entity
	// carries an AuthMethod, so the pointer cannot be shared between them.
	if r.BuiltinMethod != nil {
		identity.BuiltinMethod = lo.ToPtr(entity.AuthMethod(*r.BuiltinMethod))
	}

	return identity
}

func toDB(r *entity.UserIdentity) *model.UserIdentities {
	row := &model.UserIdentities{
		UserID:        r.UserID,
		IntegrationID: r.IntegrationID,
		Subject:       r.Subject,
		Email:         r.Email,
	}

	if r.BuiltinMethod != nil {
		row.BuiltinMethod = lo.ToPtr(string(*r.BuiltinMethod))
	}

	return row
}
