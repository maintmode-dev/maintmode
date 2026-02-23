package apismodels

import (
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

type Resource struct {
	ID          uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-4466554400000"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ExternalID  *string    `json:"external_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at" format:"date-time"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" format:"date-time"`
}

type CreateResourceRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ExternalID  *string `json:"external_id,omitempty"`
}

type ResourceType struct {
	Type entity.ResourceType `json:"type"`
}

type GetResourceTypesResponse struct {
	Types []*ResourceType `json:"types"`
}

type SearchResourcesRequest struct {
	Name string `query:"name" validate:"required"`
}

type SearchResourcesResponse struct {
	Items []*Resource `json:"resources"`
}
