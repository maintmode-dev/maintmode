package resources

import (
	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
)

func fromDBResource(r *model.Resources) *entity.ResourceDetails {
	return &entity.ResourceDetails{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		ExternalID:  r.ExternalID,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func uuidsToPgUUID(resourceIDs []uuid.UUID) []postgres.StringExpression {
	return lo.Map(resourceIDs, func(item uuid.UUID, _ int) postgres.StringExpression {
		return postgres.UUID(item)
	})
}
