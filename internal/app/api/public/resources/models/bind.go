package apimodels

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

func ToAPIResource(r *entity.ResourceDetails) *Resource {
	return &Resource{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		ExternalID:  r.ExternalID,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func ToAPIResources(resources []*entity.ResourceDetails) []*Resource {
	return lo.Map(resources, func(r *entity.ResourceDetails, _ int) *Resource {
		return ToAPIResource(r)
	})
}
