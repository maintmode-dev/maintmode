package resources

import (
	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func toDBResource(r *entity.ResourceDetails) *model.Resources {
	return &model.Resources{
		ID:              r.ID,
		Name:            r.Name,
		Description:     r.Description,
		ExternalID:      r.ExternalID,
		Status:          string(r.Status),
		CreatedAt:       r.CreatedAt,
		CreatedByUserID: r.CreatedByUserID,
		UpdatedAt:       r.UpdatedAt,
		UpdatedByUserID: r.UpdatedByUserID,
	}
}

func fromDBResource(r *model.Resources) *entity.ResourceDetails {
	return &entity.ResourceDetails{
		ID:              r.ID,
		Name:            r.Name,
		Description:     r.Description,
		ExternalID:      r.ExternalID,
		Status:          entity.ResourceStatus(r.Status),
		CreatedAt:       r.CreatedAt,
		CreatedByUserID: r.CreatedByUserID,
		UpdatedAt:       r.UpdatedAt,
		UpdatedByUserID: r.UpdatedByUserID,
	}
}

func uuidsToPgUUID(resourceIDs []uuid.UUID) []postgres.StringExpression {
	return lo.Map(resourceIDs, func(item uuid.UUID, _ int) postgres.StringExpression {
		return postgres.UUID(item)
	})
}
